package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// Create a new video.
func createVideo(w http.ResponseWriter, r *http.Request) error {
	var (
		data     VideoRequest
		uploadId string
	)

	if err := parseJSON(w, r, &data); err != nil {
		return fmt.Errorf("cannot parse JSON from request body: %v", err)
	}
	if r.Header.Get("X-Upload-Content-Length") == "" {
		return &AppError{http.StatusBadRequest, "X-Upload-Content-Length header must be required"}
	}
	if r.Header.Get("X-Upload-Content-Type") == "" {
		return &AppError{http.StatusBadRequest, "X-Upload-Content-Type header must be required"}
	}

	id := uuid.New().String()
	sess := session.Must(session.NewSession())
	uploader := s3manager.NewUploader(sess)
	metadata := map[string]string{
		"title":       data.Metadata.Title,
		"description": data.Metadata.Description,
		"length":      r.Header.Get("X-Upload-Content-Length"),
		"tags":        strings.Join(data.Metadata.Tags, ","),
		"type":        r.Header.Get("X-Upload-Content-Type"),
	}

	switch r.URL.Query().Get("uploadType") {
	case "media":
	case "resumable":
		// Initiates a multipart upload and return an upload ID from AWS S3.
		out, err := uploader.S3.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
			Bucket:   aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
			Key:      aws.String(id),
			Metadata: aws.StringMap(metadata),
		})
		if err != nil {
			return fmt.Errorf("failed to create multipart upload: %v", err)
		}
		uploadId = *out.UploadId
	default:
		return &AppError{http.StatusBadRequest, "Invalid upload type"}
	}

	// Save the multipart file information to the persistence data store.
	db := dynamodb.New(sess)
	_, err := db.PutItem(&dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"Id": {S: aws.String(id)},
			"Metadata": {
				M: map[string]*dynamodb.AttributeValue{
					"Title":       {S: aws.String(metadata["title"])},
					"Description": {S: aws.String(metadata["description"])},
					"Length":      {N: aws.String(metadata["length"])},
					"Tags":        {S: aws.String(metadata["tags"])},
					"Type":        {S: aws.String(metadata["type"])},
				},
			},
			"UploadId": {
				S: aws.String(uploadId),
			},
			"CreatedAt": {
				N: aws.String(strconv.FormatInt(time.Now().Unix(), 10)),
			},
		},
		TableName: aws.String(os.Getenv("AWS_DB_VOD_NAME")),
	})
	if err != nil {
		return fmt.Errorf("failed to save data to dynamodb: %v", err)
	}

	return replyJSON(w, VideoResponse{id}, http.StatusOK)
}

// Upload the video to the remote storage.
func uploadVideo(w http.ResponseWriter, r *http.Request) error {
	id := mux.Vars(r)["id"]
	if id == "" {
		return &AppError{http.StatusBadRequest, "video ID must be required"}
	}
	length, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return &AppError{http.StatusBadRequest, fmt.Sprintf("cannot parse Content-Length header: %v", err)}
	}
	if length < MinUploadChunkSize || length > MaxUploadChunkSize {
		return &AppError{http.StatusBadRequest, fmt.Sprintf("size must between %d and %d bytes", MinUploadChunkSize, MaxUploadChunkSize)}
	}
	if length%MinUploadChunkSize > 0 {
		return &AppError{http.StatusBadRequest, fmt.Sprintf("size must be the multiple of %d bytes", MinUploadChunkSize)}
	}

	var cr *httprange.ContentRange
	sess := session.Must(session.NewSession())
	db := dynamodb.New(sess)
	resp, err := db.GetItem(&dynamodb.GetItemInput{
		Key:       map[string]*dynamodb.AttributeValue{"Id": {S: aws.String(id)}},
		TableName: aws.String(os.Getenv("AWS_DB_VOD_NAME")),
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve the video file from DynamoDB: %v", err)
	}
	if len(resp.Item) == 0 {
		return &AppError{http.StatusNotFound, "video ID does not exist"}
	}
	// Parse the Content-Range header for resumable upload.
	if r.Header.Get("Content-Range") != "" {
		cr, err = httprange.ParseContentRange(r.Header.Get("Content-Range"))
		if err != nil {
			return fmt.Errorf("cannot parse Content-Range header: %v", err)
		}
		if length != cr.Length() {
			return &AppError{http.StatusBadRequest, "invalid length of Content-Range header"}
		}
		if strconv.FormatInt(cr.Size, 10) != *resp.Item["Metadata"].M["Length"].N {
			return &AppError{http.StatusBadRequest, "invalid size of Content-Range header"}
		}
	}
	uploader := s3manager.NewUploader(sess)
	body := http.MaxBytesReader(w, r.Body, MaxUploadChunkSize)
	// Upload the video file by the given upload type.
	// - media: Simple upload. Use this type to quickly transfer small media file to the remote storage.
	// - resumable: Resumable upload. Use this type for large files when there's a high chance fo network interruption.
	switch r.URL.Query().Get("uploadType") {
	case "media":
		if err := simpleUpload(id, body, uploader); err != nil {
			return err
		}
	case "resumable":
		completed, err := resumableUpload(id, *resp.Item["UploadId"].S, body, cr, uploader)
		if err != nil {
			return err
		}
		// Respond to the client if the upload was not completed.
		if !completed {
			w.WriteHeader(http.StatusPartialContent)
			return nil
		}
	default:
		return &AppError{http.StatusBadRequest, "Invalid upload type"}
	}
	// Respond in success when the given file has been uploaded.
	w.WriteHeader(http.StatusOK)
	return nil
}

// Upload an entire file to the remote storage.
func simpleUpload(id string, body io.Reader, uploader *s3manager.Uploader) error {
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, body)
	if err != nil {
		return fmt.Errorf("failed to write binary data: %v", err)
	}
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		Key:    aws.String(id),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		return fmt.Errorf("failed to upload video to S3: %v", err)
	}
	return nil
}

// Resumable upload the file into chunks.
func resumableUpload(id, uploadId string, body io.Reader, cr *httprange.ContentRange, uploader *s3manager.Uploader) (bool, error) {
	buf := new(bytes.Buffer)
	written, err := io.Copy(buf, body)
	if err != nil {
		return false, fmt.Errorf("failed to write binary data: %v", err)
	}
	_, err = uploader.S3.UploadPart(&s3.UploadPartInput{
		Body:          bytes.NewReader(buf.Bytes()),
		Bucket:        aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		ContentLength: aws.Int64(written),
		Key:           aws.String(id),
		PartNumber:    aws.Int64(cr.CurrentPart()),
		UploadId:      aws.String(uploadId),
	})
	if err != nil {
		return false, fmt.Errorf("failed to partial upload to S3, %v", err)
	}
	out, err := uploader.S3.ListParts(&s3.ListPartsInput{
		Bucket:   aws.String(os.Getenv("AWS_S3_VOD_BUCKET")),
		Key:      aws.String(id),
		UploadId: aws.String(uploadId),
	})
	if err != nil {
		return false, fmt.Errorf("failed to list multipart upload: %v", err)
	}
	if len(out.Parts) < int(cr.Parts()) {
		return false, nil
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
		UploadId: aws.String(uploadId),
	})
	if err != nil {
		return false, fmt.Errorf("failed to complete multipart upload: %v", err)
	}
	return true, nil
}

// Parse incoming request body as JSON object.
func parseJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(data); err != nil {
		return err
	}
	return nil
}

// Respond the output with JSON format to the client.
func replyJSON(w http.ResponseWriter, data interface{}, code int) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		return err
	}
	return nil
}
