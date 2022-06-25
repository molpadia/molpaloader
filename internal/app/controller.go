package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/molpadia/molpastream/internal/domain/entity"
	"github.com/molpadia/molpastream/internal/httprange"
	"github.com/molpadia/molpastream/internal/infrastructure"
	"github.com/molpadia/molpastream/internal/infrastructure/persistence"
)

const (
	minUploadChunkSize = 256 << 10
	maxUploadChunkSize = 10 << 20
)

// Get a single video.
func getVideo(w http.ResponseWriter, r *http.Request) error {
	id := mux.Vars(r)["id"]
	if id == "" {
		return &appError{http.StatusBadRequest, "video ID must be required"}
	}
	sess := session.Must(session.NewSession())
	repo := persistence.NewVideoRepository(sess)
	video, err := repo.GetById(id)
	if err != nil {
		return fmt.Errorf("failed to retrieve the video file")
	}
	if video == nil {
		return &appError{http.StatusNotFound, "video ID does not exist"}
	}
	return replyJSON(w, VideoResponse{video.Id, video.Description, video.Tags, video.Metadata, video.Status}, http.StatusOK)
}

// Create a new video.
func createVideo(w http.ResponseWriter, r *http.Request) error {
	var data VideoRequest
	if err := parseJSON(w, r, &data); err != nil {
		return fmt.Errorf("cannot parse JSON from request body: %v", err)
	}
	if r.Header.Get("X-Upload-Content-Type") == "" {
		return &appError{http.StatusBadRequest, "X-Upload-Content-Type header must be required"}
	}
	size, err := strconv.ParseInt(r.Header.Get("X-Upload-Content-Length"), 10, 64)
	if err != nil {
		return &appError{http.StatusBadRequest, "X-Upload-Content-Length header must be required"}
	}
	sess := session.Must(session.NewSession())
	uploader := infrastructure.NewUploader(sess)
	repo := persistence.NewVideoRepository(sess)
	// Create a new video entity for persistence data store.
	video := entity.NewVideo(
		uuid.New().String(),
		data.Title,
		data.Description,
		r.Header.Get("X-Upload-Content-Type"),
		size,
		data.Tags,
		data.Metadata,
	)
	switch r.URL.Query().Get("uploadType") {
	case "media":
	case "resumable":
		uploadId, err := uploader.CreateMultipart(video.Id)
		if err != nil {
			return fmt.Errorf("failed to create multipart upload: %v", err)
		}
		video.NewUpload(uploadId)
	default:
		return &appError{http.StatusBadRequest, "Invalid upload type"}
	}
	// Save the multipart file information to the persistence.
	if err = repo.Save(video); err != nil {
		return fmt.Errorf("failed to save data to dynamodb: %v", err)
	}
	return replyJSON(w, VideoResponse{video.Id, video.Description, video.Tags, video.Metadata, video.Status}, http.StatusCreated)
}

// Upload the video to the remote storage.
func uploadVideo(w http.ResponseWriter, r *http.Request) error {
	id := mux.Vars(r)["id"]
	if id == "" {
		return &appError{http.StatusBadRequest, "video ID must be required"}
	}
	// Get the partial size of video upload.
	size, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return &appError{http.StatusBadRequest, fmt.Sprintf("cannot parse Content-Length header: %v", err)}
	}
	if size < minUploadChunkSize || size > maxUploadChunkSize {
		return &appError{http.StatusBadRequest, fmt.Sprintf("size must between %d and %d bytes", minUploadChunkSize, maxUploadChunkSize)}
	}
	if size%minUploadChunkSize > 0 {
		return &appError{http.StatusBadRequest, fmt.Sprintf("size must be the multiple of %d bytes", minUploadChunkSize)}
	}
	var cr *httprange.ContentRange
	sess := session.Must(session.NewSession())
	uploader := infrastructure.NewUploader(sess)
	repo := persistence.NewVideoRepository(sess)
	video, err := repo.GetById(id)
	if err != nil {
		return fmt.Errorf("failed to retrieve the video file from DynamoDB: %v", err)
	}
	if video == nil {
		return &appError{http.StatusNotFound, "video ID does not exist"}
	}
	// Parse the Content-Range header for resumable upload.
	if r.Header.Get("Content-Range") != "" {
		cr, err = httprange.ParseContentRange(r.Header.Get("Content-Range"))
		if err != nil {
			return fmt.Errorf("cannot parse Content-Range header: %v", err)
		}
		if cr.Length() != size {
			return &appError{http.StatusBadRequest, "invalid length of Content-Range header"}
		}
		if cr.Size != video.Size {
			return &appError{http.StatusBadRequest, "invalid size of Content-Range header"}
		}
	}
	buf := new(bytes.Buffer)
	body := http.MaxBytesReader(w, r.Body, maxUploadChunkSize)
	if _, err = io.Copy(buf, body); err != nil {
		return fmt.Errorf("failed to write binary data: %v", err)
	}
	// Upload the video file by the given upload type.
	// - media: Simple upload. Use this type to quickly transfer small media file to the remote storage.
	// - resumable: Resumable upload. Use this type for large files when there's a high chance fo network interruption.
	switch r.URL.Query().Get("uploadType") {
	case "media":
		err = uploader.SimpleUpload(id, buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to upload video to S3: %v", err)
		}
		video.SetStatus(entity.UploadedStatusCompleted)
		if err = repo.Save(video); err != nil {
			return fmt.Errorf("failed to save video to the persistence")
		}
	case "resumable":
		if cr != nil {
			return &appError{http.StatusBadRequest, "Content-Range must be required"}
		}
		part, err := uploader.UploadPart(id, video.Upload.Id, buf.Bytes(), size, cr.CurrentPart())
		if err != nil {
			return fmt.Errorf("failed to partial upload to S3, %v", err)
		}
		video.AddUploadPart(part)
		// Assemble uploaded parts and complete the upload.
		if len(video.Upload.Parts) >= int(cr.Parts()) {
			video.SetStatus(entity.UploadedStatusCompleted)
			if err = uploader.CompleteMultipart(id, video.Upload.Id, video.Upload.Parts); err != nil {
				return fmt.Errorf("failed to complete multipart upload: %v", err)
			}
		}
		if err = repo.Save(video); err != nil {
			return fmt.Errorf("failed to save video to the persistence")
		}
	default:
		return &appError{http.StatusBadRequest, "Invalid upload type"}
	}
	// Respond to the client if the upload was not completed.
	if cr != nil && len(video.Upload.Parts) < int(cr.Parts()) {
		w.WriteHeader(http.StatusPartialContent)
		return nil
	}
	// Respond in success when the given file has been uploaded.
	w.WriteHeader(http.StatusOK)
	return nil
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
