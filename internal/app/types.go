package app

type VideoRequest struct {
	Metadata Metadata `json:"metadata"`
}

type VideoResponse struct {
	Id string `json:"id"`
}

type Metadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}
