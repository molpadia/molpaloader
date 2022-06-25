package app

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/molpadia/molpastream/internal/domain/entity"
)

func TestGetVideo(t *testing.T) {
	tests := []struct {
		path        string
		vars        map[string]string
		video       *entity.Video
		expectedErr error
	}{
		{"/molpastream/v1/videos", map[string]string{}, nil, errors.New("video ID must be required")},
		{"/molpastream/v1/videos/1", map[string]string{"id": "1"}, nil, errors.New("video ID does not exist")},
		{"/molpastream/v1/videos/1", map[string]string{"id": "1"}, &entity.Video{}, nil},
	}
	for _, tt := range tests {
		r, err := http.NewRequest("GET", tt.path, bytes.NewBuffer(nil))
		if err != nil {
			t.Fatal(err)
		}
		r = mux.SetURLVars(r, tt.vars)
		w := httptest.NewRecorder()
		c := &controller{&mockVideoRepoistory{tt.video}, &mockUploader{}}
		err = c.getVideo(w, r)
		if !errors.Is(err, tt.expectedErr) {
			t.Errorf("expected error (%v), got error (%v)", tt.expectedErr, err)
		}
	}
}

func TestCreateVideo(t *testing.T) {
	tests := []struct {
		body        string
		headers     http.Header
		path        string
		expectedErr error
	}{
		{"{}", map[string][]string{}, "/molpastream/v1/videos", errors.New("X-Upload-Content-Type header must be required")},
		{"{}", map[string][]string{"X-Upload-Content-Type": {"video/mp4"}}, "/molpastream/v1/videos", errors.New("X-Upload-Content-Length header must be required")},
		{"{}", map[string][]string{"X-Upload-Content-Type": {"video/mp4"}, "X-Upload-Content-Length": {"41943040"}}, "/molpastream/v1/videos", errors.New("Invalid upload type")},
		{"{}", map[string][]string{"X-Upload-Content-Type": {"video/mp4"}, "X-Upload-Content-Length": {"41943040"}}, "/molpastream/v1/videos?uploadType=", errors.New("Invalid upload type")},
		{"{}", map[string][]string{"X-Upload-Content-Type": {"video/mp4"}, "X-Upload-Content-Length": {"41943040"}}, "/molpastream/v1/videos?uploadType=media", nil},
		{"{}", map[string][]string{"X-Upload-Content-Type": {"video/mp4"}, "X-Upload-Content-Length": {"41943040"}}, "/molpastream/v1/videos?uploadType=resumable", nil},
	}
	for _, tt := range tests {
		r, err := http.NewRequest("POST", tt.path, bytes.NewBuffer([]byte(tt.body)))
		if err != nil {
			t.Fatal(err)
		}
		r.Header = tt.headers
		w := httptest.NewRecorder()
		c := &controller{&mockVideoRepoistory{}, &mockUploader{}}
		err = c.createVideo(w, r)
		if !errors.Is(err, tt.expectedErr) {
			t.Errorf("expected error (%v), got error (%v)", tt.expectedErr, err)
		}
	}
}

func TestUploadVideo(t *testing.T) {
	tests := []struct {
		headers     http.Header
		query       string
		video       *entity.Video
		expectedErr error
	}{
		{map[string][]string{}, "", nil, errors.New(`cannot parse Content-Length header: strconv.ParseInt: parsing "": invalid syntax`)},
		{map[string][]string{"Content-Length": {"-1"}}, "", nil, fmt.Errorf("size must between %d and %d bytes", minUploadChunkSize, maxUploadChunkSize)},
		{map[string][]string{"Content-Length": {"10485761"}}, "", nil, fmt.Errorf("size must between %d and %d bytes", minUploadChunkSize, maxUploadChunkSize)},
		{map[string][]string{"Content-Length": {"262145"}}, "", nil, fmt.Errorf("size must be the multiple of %d bytes", minUploadChunkSize)},
		{map[string][]string{"Content-Length": {"1048576"}}, "", nil, errors.New("video ID does not exist")},
		{map[string][]string{"Content-Length": {"1048576"}}, "", &entity.Video{}, errors.New("Invalid upload type")},
		{map[string][]string{"Content-Length": {"1048576"}}, "uploadType=media", &entity.Video{}, nil},
		{map[string][]string{"Content-Length": {"1048576"}}, "uploadType=resumable", &entity.Video{}, errors.New("Content-Range must be required")},
		{map[string][]string{"Content-Length": {"1048576"}}, "uploadType=resumable", &entity.Video{}, errors.New("Content-Range must be required")},
		{map[string][]string{"Content-Length": {"1048576"}, "Content-Range": {"bytes 0-1048575/10485760"}}, "uploadType=resumable", &entity.Video{Size: 10485760, Upload: &entity.UploadProgress{Id: "1"}}, nil},
		{map[string][]string{"Content-Length": {"1048576"}, "Content-Range": {"bytes 9437184-10485759/10485760"}}, "uploadType=resumable", &entity.Video{Size: 10485760, Upload: &entity.UploadProgress{Id: "1"}}, nil},
	}
	for _, tt := range tests {
		r, err := http.NewRequest("PUT", fmt.Sprintf("/upload/molpastream/v1/videos/1?%s", tt.query), bytes.NewBuffer(nil))
		if err != nil {
			t.Fatal(err)
		}
		r.Header = tt.headers
		r = mux.SetURLVars(r, map[string]string{"id": "1"})
		w := httptest.NewRecorder()
		c := &controller{&mockVideoRepoistory{tt.video}, &mockUploader{}}
		err = c.uploadVideo(w, r)
		if !errors.Is(err, tt.expectedErr) {
			t.Errorf("expected error (%v), got error (%v)", tt.expectedErr, err)
		}
	}
}

type mockVideoRepoistory struct {
	video *entity.Video
}

func (r *mockVideoRepoistory) GetById(id string) (*entity.Video, error) {
	return r.video, nil
}

func (r *mockVideoRepoistory) Save(video *entity.Video) error {
	r.video = video
	return nil
}

type mockUploader struct {
}

func (u *mockUploader) CreateMultipart(key string) (string, error) {
	return key, nil
}

func (u *mockUploader) CompleteMultipart(key, uploadId string, parts []*entity.Part) error {
	return nil
}

func (u *mockUploader) SimpleUpload(key string, body []byte) error {
	return nil
}

func (u *mockUploader) UploadPart(key, uploadId string, body []byte, length, partNumber int64) (*entity.Part, error) {
	return &entity.Part{ETag: "b54357faf0632cce46e942fa68356b38", PartNumber: partNumber}, nil
}
