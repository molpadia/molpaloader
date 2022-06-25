package repository

import "github.com/molpadia/molpastream/internal/domain/entity"

type VideoRepository interface {
	// Get the video by the video ID.
	GetById(id string) (*entity.Video, error)
	// Save an entity to the persistence.
	Save(video *entity.Video) error
}
