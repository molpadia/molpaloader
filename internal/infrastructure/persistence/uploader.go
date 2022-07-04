package persistence

import (
	"bytes"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/molpadia/molpastream/internal/domain/entity"
)

var bucket = os.Getenv("AWS_VOD_BUCKET")

type Uploader struct {
	s3Uploader *s3manager.Uploader
}

func NewUploader(sess *session.Session) *Uploader {
	return &Uploader{s3manager.NewUploader(sess)}
}

// Initiates a multipart upload and return an upload ID from remote AWS S3 storage.
func (u *Uploader) CreateMultipart(key string) (string, error) {
	out, err := u.s3Uploader.S3.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return *out.UploadId, err
}

// Mark the multipart upload as completd for the remote AWS S3 storage.
func (u *Uploader) CompleteMultipart(key, uploadId string, parts []*entity.Part) error {
	var fileParts []*s3.CompletedPart
	for _, part := range parts {
		fileParts = append(fileParts, &s3.CompletedPart{
			ETag:       aws.String(part.ETag),
			PartNumber: aws.Int64(part.PartNumber),
		})
	}
	_, err := u.s3Uploader.S3.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: fileParts,
		},
		UploadId: aws.String(uploadId),
	})
	return err
}

// Upload an entire file to remote AWS S3 storage.
func (u *Uploader) SimpleUpload(key string, body []byte) error {
	_, err := u.s3Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

// Upload a file part to remote AWS S3 storage.
func (u *Uploader) UploadPart(key, uploadId string, body []byte, length, partNumber int64) (*entity.Part, error) {
	out, err := u.s3Uploader.S3.UploadPart(&s3.UploadPartInput{
		Body:          bytes.NewReader(body),
		Bucket:        aws.String(bucket),
		ContentLength: aws.Int64(length),
		Key:           aws.String(key),
		PartNumber:    aws.Int64(partNumber),
		UploadId:      aws.String(uploadId),
	})
	if err != nil {
		return nil, err
	}
	return &entity.Part{ETag: *out.ETag, PartNumber: partNumber}, nil
}
