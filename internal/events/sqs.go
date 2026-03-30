package events

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Queue names — each write operation has its own dedicated queue
const (
	CreateQueue = "user-create-queue"
	UpdateQueue = "user-update-queue"
	DeleteQueue = "user-delete-queue"
)

func getAWSEndpoint() string {
	lsHostname := os.Getenv("LOCALSTACK_HOSTNAME")
	if lsHostname != "" {
		return fmt.Sprintf("http://%s:4566", lsHostname)
	}
	awsEndpoint := os.Getenv("AWS_ENDPOINT_URL")
	if awsEndpoint != "" {
		return awsEndpoint
	}
	return "http://localstack_project:4566"
}

// EmitEvent publishes a sync trigger message to the specified queue.
func EmitEvent(ctx context.Context, queueName string, operation string, userID int) {
	awsEndpoint := getAWSEndpoint()

	// Allow per-queue override via env var, fallback to constructed URL
	queueURL := os.Getenv("SQS_" + operation + "_QUEUE_URL")
	if queueURL == "" {
		queueURL = fmt.Sprintf(
			"http://sqs.us-east-1.localhost.localstack.cloud:4566/000000000000/%s",
			queueName,
		)
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               awsEndpoint,
			SigningRegion:     "us-east-1",
			HostnameImmutable: true,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		log.Printf("EVENT_EMIT_FAILURE: failed to load AWS config: %v", err)
		return
	}

	client := sqs.NewFromConfig(cfg)
	body := fmt.Sprintf(`{"operation":"%s","user_id":%d}`, operation, userID)

	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    &queueURL,
		MessageBody: aws.String(body),
	})
	if err != nil {
		log.Printf("EVENT_EMIT_FAILURE [%s]: %v (data is still safe in DB)", queueName, err)
	} else {
		log.Printf("Event emitted to %s: operation=%s userID=%d", queueName, operation, userID)
	}
}
