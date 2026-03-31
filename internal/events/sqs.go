package events

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/domain"
	"project-serverless/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

// Stream names
const (
	DomainStream = "user-domain-events"
	AuditStream  = "user-audit-events"
)

func getAWSEndpoint() string {
	lsHostname := os.Getenv("LOCALSTACK_HOSTNAME")
	if lsHostname != "" {
		return "http://" + lsHostname + ":4566"
	}
	awsEndpoint := os.Getenv("AWS_ENDPOINT_URL")
	if awsEndpoint != "" {
		return awsEndpoint
	}
	return "http://localstack_project:4566"
}

func EmitUserEvents(ctx context.Context, operation string, user domain.User) error {
	awsEndpoint := getAWSEndpoint()
	domainStream := os.Getenv("KINESIS_DOMAIN_STREAM_NAME")
	if domainStream == "" {
		domainStream = DomainStream
	}
	auditStream := os.Getenv("KINESIS_AUDIT_STREAM_NAME")
	if auditStream == "" {
		auditStream = AuditStream
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
		logger.Error("event_emit_failure", map[string]any{"stage": "load_aws_config", "error": err.Error()})
		return apperrors.NewInternal("event emit failed: load aws config", err)
	}

	client := kinesis.NewFromConfig(cfg)
	partitionKey := strconv.Itoa(user.ID)

	domainPayload, err := json.Marshal(UserEventPayload{
		EventType: "domain",
		Entity:    "user",
		Operation: operation,
		User: UserSnapshot{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return apperrors.NewInternal("failed to marshal domain event", err)
	}

	auditPayload, err := json.Marshal(UserEventPayload{
		EventType: "audit",
		Entity:    "user",
		Operation: operation,
		User: UserSnapshot{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return apperrors.NewInternal("failed to marshal audit event", err)
	}

	if _, err = client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(domainStream),
		Data:         domainPayload,
		PartitionKey: aws.String(partitionKey),
	}); err != nil {
		return apperrors.NewInternal("failed to publish domain event", err)
	}

	if _, err = client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(auditStream),
		Data:         auditPayload,
		PartitionKey: aws.String(partitionKey),
	}); err != nil {
		return apperrors.NewInternal("failed to publish audit event", err)
	}

	logger.Info("events_emitted_to_kinesis", map[string]any{
		"operation":     operation,
		"user_id":       user.ID,
		"domain_stream": domainStream,
		"audit_stream":  auditStream,
	})
	return nil
}
