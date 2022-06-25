package repository

import "github.com/molpadia/molpastream/internal/domain/entity"

type Uploader interface {
	// Initiates a multipart upload and return an upload ID from remote AWS S3 storage.
	CreateMultipart(key string) (string, error)
	// Mark the multipart upload as completd for the remote AWS S3 storage.
	CompleteMultipart(key, uploadId string, parts []*entity.Part) error
	// Upload an entire file to remote AWS S3 storage.
	SimpleUpload(key string, body []byte) error
	// Upload a file part to remote AWS S3 storage.
	UploadPart(key, uploadId string, body []byte, length, partNumber int64) (*entity.Part, error)
}
