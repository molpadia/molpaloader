package app

type VideoRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
}

type VideoResponse struct {
	Id          string            `json:"id"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Metadata    map[string]string `json:"metadata"`
	Status      string            `json:"status"`
}
