package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/mediaconvert"
)

const jobSettingPath = "job.json"

// Get URI path for a file stored in S3 bucket.
func s3Path(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

// Load the configuration of mediaconvert job settings.
func loadJobSettings() (*mediaconvert.JobSettings, error) {
	buf, err := os.ReadFile(jobSettingPath)
	if err != nil {
		log.Printf("failed to load job setting file: %v", err)
		return nil, err
	}
	var js *mediaconvert.JobSettings
	if err = json.Unmarshal(buf, &js); err != nil {
		log.Printf("failed to unmarshal job settings, %v", err)
		return nil, err
	}
	return js, nil
}

// Invoke the AWS Lambda function to trancode the given video to outputs.
func handler(ctx context.Context, event events.S3Event) error {
	bucket := event.Records[0].S3.Bucket.Name
	key := event.Records[0].S3.Object.Key
	js, err := loadJobSettings()
	if err != nil {
		return err
	}
	js.Inputs[0].FileInput = aws.String(s3Path(bucket, key))
	js.OutputGroups[0].OutputGroupSettings.HlsGroupSettings.Destination = aws.String(s3Path(os.Getenv("AWS_VOD_HLS_BUCKET"), key))
	// Create a mediaconvert job
	mc := mediaconvert.New(session.Must(session.NewSession(&aws.Config{
		Endpoint: aws.String(os.Getenv("AWS_VOD_MEDIACONVERT_URL")),
	})))
	out, err := mc.CreateJob(&mediaconvert.CreateJobInput{
		Role:     aws.String(os.Getenv("AWS_VOD_ROLE_ARN")),
		Settings: js,
	})
	if err != nil {
		log.Printf("failed to launch mediaconvert job: %v", err)
		return err
	}
	log.Printf("mediaconvert job launched %v", out)
	return nil
}

func main() {
	lambda.Start(handler)
}
