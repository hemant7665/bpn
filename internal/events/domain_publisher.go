package events

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	svcerrors "project-serverless/internal/errors"
)

// DomainEventsStreamEnv selects the Kinesis stream for user domain events (BluePrint: blueprint-events).
const DomainEventsStreamEnv = "KINESIS_DOMAIN_EVENTS_STREAM_NAME"

// DomainEventPublisher sends projection/domain payloads to Kinesis (downstream from userEventWorker).
type DomainEventPublisher struct {
	client *kinesis.Client
	stream string
}

func NewDomainEventPublisher(ctx context.Context) (*DomainEventPublisher, error) {
	stream := os.Getenv(DomainEventsStreamEnv)
	if stream == "" {
		stream = "user-domain-events"
	}

	awsEndpoint := getAWSEndpoint()
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
		return nil, svcerrors.Internal("domain event publisher: load aws config", err)
	}

	return &DomainEventPublisher{
		client: kinesis.NewFromConfig(cfg),
		stream: stream,
	}, nil
}

// PutRecordJSON sends one record (payload must be JSON bytes).
func (p *DomainEventPublisher) PutRecordJSON(ctx context.Context, partitionKey string, payload []byte) error {
	if partitionKey == "" {
		partitionKey = "user-domain"
	}
	if _, err := p.client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(p.stream),
		Data:         payload,
		PartitionKey: aws.String(partitionKey),
	}); err != nil {
		return svcerrors.Internal("domain event publisher: PutRecord", err)
	}
	return nil
}
