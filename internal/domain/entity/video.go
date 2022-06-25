package entity

const (
	UploadedStatusCompleted = "UPLOADED"
	UploadedStatusDeleted   = "DELETED"
	UploadedStatusFailed    = "FAILED"
	UploadedStatusProcessed = "PROCESSED"
	UploadedStatusRejected  = "REJECTED"
)

// The entity of stream video.
type Video struct {
	Id          string
	ContentType string
	Description string
	Metadata    map[string]string
	Tags        []string
	Title       string
	Size        int64
	Status      string
	Upload      *UploadProgress
}

func NewVideo(id, title, description, contentType string, size int64, tags []string, metadata map[string]string) *Video {
	return &Video{
		Id:          id,
		Title:       title,
		Description: description,
		ContentType: contentType,
		Size:        size,
		Status:      UploadedStatusProcessed,
		Tags:        tags,
		Metadata:    metadata,
	}
}

func (v *Video) NewUpload(id string) { v.Upload = &UploadProgress{Id: id} }

// Add a file part to video for multipart upload.
func (v *Video) AddUploadPart(part *Part) {
	v.Upload.Parts = append(v.Upload.Parts, part)
}

// Mark the upload status to the video.
func (v *Video) SetStatus(status string) {
	v.Status = status
}

// The uplaod progress is used for multipart upload.
type UploadProgress struct {
	Id    string  // The upload identifier in multipart upload.
	First int64   // The first byte was uploaded to the storage.
	Last  int64   // The last byte was uploaded to the storage.
	Parts []*Part // A set of parts in multipart upload.
}

// The part portion of video data.
type Part struct {
	ETag       string // Entity tag for the uploaded object.
	PartNumber int64  // Part number that identifies the part.
}
