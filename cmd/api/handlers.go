package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/molpadia/molpastream/internal/httprange"
)

const (
	MinUploadChunkSize = 256 << 10
	MaxUploadChunkSize = 5 << 20
)

type CreateVideo struct {
	Metadata Metadata `json:"metadata"`
}

type Video struct {
	Id string `json:"id"`
}

type Metadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// Create a new video.
// Initiates a multipart upload and return an upload ID from AWS S3 if upload type is resumable.
func createVideo(w http.ResponseWriter, r *http.Request) error {
	var data CreateVideo

	if err := parse(w, r, &data); err != nil {
		return fmt.Errorf("cannot parse JSON from request body: %v", err)
	}

	if r.Header.Get("X-Upload-Content-Length") == "" {
		return &appError{http.StatusBadRequest, "X-Upload-Content-Length header must be required"}
	}

	if r.Header.Get("X-Upload-Content-Type") == "" {
		return &appError{http.StatusBadRequest, "X-Upload-Content-Type header must be required"}
	}

	id := uuid.New().String()
	sess := session.Must(session.NewSession())
	metadata := map[string]string{
		"title":       data.Metadata.Title,
		"description": data.Metadata.Description,
		"length":      r.Header.Get("X-Upload-Content-Length"),
		"tags":        strings.Join(data.Metadata.Tags, ","),
		"type":        r.Header.Get("X-Upload-Content-Type"),
	}

	uploader := s3manager.NewUploader(sess)
	out, err := uploader.S3.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket:   aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		Key:      aws.String(id),
		Metadata: aws.StringMap(metadata),
	})

	if err != nil {
		log.Printf("failed to create multipart upload: %v", err)
		return err
	}

	log.Printf("initiates a multipart upload: %+v", out)

	// Save the multipart file information to the persistence data store.
	svc := dynamodb.New(sess)
	_, err = svc.PutItem(&dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(id),
			},
			"Metadata": {
				M: map[string]*dynamodb.AttributeValue{
					"Title": {
						S: aws.String(metadata["title"]),
					},
					"Description": {
						S: aws.String(metadata["description"]),
					},
					"Length": {
						N: aws.String(metadata["length"]),
					},
					"Tags": {
						S: aws.String(metadata["tags"]),
					},
					"Type": {
						S: aws.String(metadata["type"]),
					},
				},
			},
			"UploadId": {
				S: out.UploadId,
			},
			"CreatedAt": {
				N: aws.String(strconv.FormatInt(time.Now().Unix(), 10)),
			},
		},
		TableName: aws.String(os.Getenv("AWS_DB_VOD_NAME")),
	})

	if err != nil {
		log.Printf("failed to save data to dynamodb: %v", err)
		return err
	}

	response(w, http.StatusOK, Video{
		Id: id,
	})
	return nil
}

// Upload a video file in chunks.
func uploadVideo(w http.ResponseWriter, r *http.Request) error {
	body := http.MaxBytesReader(w, r.Body, MaxUploadChunkSize)
	id := mux.Vars(r)["id"]
	if id == "" {
		return &appError{http.StatusBadRequest, "video ID must be required"}
	}
	length, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse Content-Length header: %v", err)
	}
	if length > MaxUploadChunkSize {
		return &appError{http.StatusBadRequest, fmt.Sprintf("size must be less than %d bytes", MaxUploadChunkSize)}
	}
	if length < MinUploadChunkSize {
		return &appError{http.StatusBadRequest, fmt.Sprintf("size must be greater than %d bytes", MinUploadChunkSize)}
	}
	if length%MinUploadChunkSize > 0 {
		return &appError{http.StatusBadRequest, fmt.Sprintf("size must be the multiple of %d bytes", MinUploadChunkSize)}
	}
	cr, err := httprange.ParseContentRange(r.Header.Get("Content-Range"))
	if err != nil {
		return fmt.Errorf("cannot parse Content-Range header: %v", err)
	}
	if length != cr.Length() {
		return &appError{http.StatusBadRequest, "invalid length of Content-Range header"}
	}

	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)
	resp, err := svc.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(os.Getenv("AWS_DB_VOD_NAME")),
	})
	if err != nil {
		log.Printf("failed to retrieve the video file from DynamoDB: %v", err)
		return err
	}
	if len(resp.Item) == 0 {
		return &appError{http.StatusNotFound, "video ID does not exist"}
	}
	if strconv.FormatInt(cr.Size, 10) != *resp.Item["Metadata"].M["Length"].N {
		return &appError{http.StatusBadRequest, "invalid size of Content-Range header"}
	}

	buf := new(bytes.Buffer)
	written, err := io.Copy(buf, body)
	if err != nil {
		log.Printf("failed to write binary data: %v", err)
		return err
	}
	uploader := s3manager.NewUploader(sess)
	_, err = uploader.S3.UploadPart(&s3.UploadPartInput{
		Body:          bytes.NewReader(buf.Bytes()),
		Bucket:        aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		ContentLength: aws.Int64(written),
		Key:           aws.String(id),
		PartNumber:    aws.Int64(cr.CurrentPart()),
		UploadId:      resp.Item["UploadId"].S,
	})
	if err != nil {
		log.Printf("failed to partial upload to S3, %v", err)
		return err
	}

	out, err := uploader.S3.ListParts(&s3.ListPartsInput{
		Bucket:   aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		Key:      aws.String(id),
		UploadId: resp.Item["UploadId"].S,
	})
	if err != nil {
		log.Printf("failed to list multipart upload: %v", err)
		return err
	}
	// Respond to the client if the upload was not completed.
	if len(out.Parts) < int(cr.Parts()) {
		response(w, http.StatusPartialContent, struct{}{})
		return nil
	}

	var parts []*s3.CompletedPart
	for _, part := range out.Parts {
		parts = append(parts, &s3.CompletedPart{
			ETag:       part.ETag,
			PartNumber: part.PartNumber,
		})
	}
	_, err = uploader.S3.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
		Bucket: aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		Key:    aws.String(id),
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: parts,
		},
		UploadId: resp.Item["UploadId"].S,
	})
	if err != nil {
		log.Printf("failed to complete multipart upload: %v", err)
		return err
	}
	response(w, http.StatusCreated, struct{}{})
	return nil
}
