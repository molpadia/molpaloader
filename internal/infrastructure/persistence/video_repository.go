package persistence

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/molpadia/molpastream/internal/domain/entity"
)

var tableName = os.Getenv("AWS_DB_VOD_NAME")

type VideoRepository struct {
	db *dynamodb.DynamoDB
}

func NewVideoRepository(sess *session.Session) *VideoRepository {
	return &VideoRepository{dynamodb.New(sess)}
}

// Get the video by the video ID.
func (r *VideoRepository) GetById(id string) (*entity.Video, error) {
	out, err := r.db.GetItem(&dynamodb.GetItemInput{
		Key:       map[string]*dynamodb.AttributeValue{"Id": {S: aws.String(id)}},
		TableName: aws.String(tableName),
	})
	if err != nil || len(out.Item) == 0 {
		return nil, err
	}
	var video *entity.Video
	err = dynamodbattribute.UnmarshalMap(out.Item, &video)
	return video, err
}

// Save an entity to the persistence.
func (r *VideoRepository) Save(video *entity.Video) error {
	av, err := dynamodbattribute.MarshalMap(video)
	if err != nil {
		return err
	}
	_, err = r.db.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Printf("failed to save persistence: %v", av)
	}
	return err
}
