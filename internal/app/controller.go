package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/molpadia/molpastream/internal/domain/entity"
	"github.com/molpadia/molpastream/internal/domain/repository"
	"github.com/molpadia/molpastream/internal/httprange"
)

const (
	minUploadChunkSize = 256 << 10
	maxUploadChunkSize = 10 << 20
)

type controller struct {
	video_repo repository.VideoRepository
	uploader   repository.Uploader
}

// Get a single video.
func (c *controller) getVideo(w http.ResponseWriter, r *http.Request) error {
	id := mux.Vars(r)["id"]
	if id == "" {
		return &appError{http.StatusBadRequest, "video ID must be required"}
	}
	video, err := c.video_repo.GetById(id)
	if err != nil {
		return &appError{http.StatusInternalServerError, err.Error()}
	}
	if video == nil {
		return &appError{http.StatusNotFound, "video ID does not exist"}
	}
	return replyJSON(w, VideoResponse{video.Id, video.Description, video.Tags, video.Metadata, video.Status}, http.StatusOK)
}

// Create a new video.
func (c *controller) createVideo(w http.ResponseWriter, r *http.Request) error {
	var data VideoRequest
	if err := parseJSON(w, r, &data); err != nil {
		return &appError{http.StatusBadRequest, fmt.Sprintf("cannot parse JSON from request body: %v", err)}
	}
	if r.Header.Get("X-Upload-Content-Type") == "" {
		return &appError{http.StatusBadRequest, "X-Upload-Content-Type header must be required"}
	}
	size, err := strconv.ParseInt(r.Header.Get("X-Upload-Content-Length"), 10, 64)
	if err != nil {
		return &appError{http.StatusBadRequest, "X-Upload-Content-Length header must be required"}
	}
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
		uploadId, err := c.uploader.CreateMultipart(video.Id)
		if err != nil {
			return &appError{http.StatusInternalServerError, err.Error()}
		}
		video.NewUpload(uploadId)
	default:
		return &appError{http.StatusBadRequest, "Invalid upload type"}
	}
	// Save the multipart file information to the persistence.
	if err = c.video_repo.Save(video); err != nil {
		return &appError{http.StatusInternalServerError, err.Error()}
	}
	return replyJSON(w, VideoResponse{video.Id, video.Description, video.Tags, video.Metadata, video.Status}, http.StatusCreated)
}

// Upload the video to the remote storage.
func (c *controller) uploadVideo(w http.ResponseWriter, r *http.Request) error {
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
	video, err := c.video_repo.GetById(id)
	if err != nil {
		return &appError{http.StatusInternalServerError, err.Error()}
	}
	if video == nil {
		return &appError{http.StatusNotFound, "video ID does not exist"}
	}
	// Parse the Content-Range header for resumable upload.
	if r.Header.Get("Content-Range") != "" {
		cr, err = httprange.ParseContentRange(r.Header.Get("Content-Range"))
		if err != nil {
			return &appError{http.StatusInternalServerError, err.Error()}
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
		return &appError{http.StatusInternalServerError, err.Error()}
	}
	// Upload the video file by the given upload type.
	// - media: Simple upload. Use this type to quickly transfer small media file to the remote storage.
	// - resumable: Resumable upload. Use this type for large files when there's a high chance fo network interruption.
	switch r.URL.Query().Get("uploadType") {
	case "media":
		err = c.uploader.SimpleUpload(id, buf.Bytes())
		if err != nil {
			return &appError{http.StatusInternalServerError, err.Error()}
		}
		video.SetStatus(entity.UploadedStatusCompleted)
		if err = c.video_repo.Save(video); err != nil {
			return &appError{http.StatusInternalServerError, err.Error()}
		}
	case "resumable":
		if cr == nil {
			return &appError{http.StatusBadRequest, "Content-Range must be required"}
		}
		part, err := c.uploader.UploadPart(id, video.Upload.Id, buf.Bytes(), size, cr.CurrentPart())
		if err != nil {
			return &appError{http.StatusInternalServerError, err.Error()}
		}
		video.AddUploadPart(part)
		// Assemble uploaded parts and complete the upload.
		if len(video.Upload.Parts) >= int(cr.Parts()) {
			video.SetStatus(entity.UploadedStatusCompleted)
			if err = c.uploader.CompleteMultipart(id, video.Upload.Id, video.Upload.Parts); err != nil {
				return &appError{http.StatusInternalServerError, err.Error()}
			}
		}
		if err = c.video_repo.Save(video); err != nil {
			return &appError{http.StatusInternalServerError, err.Error()}
		}
	default:
		return &appError{http.StatusBadRequest, "Invalid upload type"}
	}
	// Respond to the client if the upload was not completed,
	// otherwise respond in success when the given file has been uploaded.
	if cr != nil && len(video.Upload.Parts) < int(cr.Parts()) {
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
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
