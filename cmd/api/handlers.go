package main

import (
	"fmt"
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
