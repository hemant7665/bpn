package events

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
)

const (
	// AuditStream is the default audit Kinesis stream name.
	AuditStream = "user-audit-events"
)

func getAWSEndpoint() string {
	if lsHostname := os.Getenv("LOCALSTACK_HOSTNAME"); lsHostname != "" {
		return "http://" + lsHostname + ":4566"
	}
	if awsEndpoint := os.Getenv("AWS_ENDPOINT_URL"); awsEndpoint != "" {
		return awsEndpoint
	}
	return "http://localstack_project:4566"
}

// AuditEmitter publishes audit trail records to the audit Kinesis stream (not the CDC stream).
type AuditEmitter struct {
	client     *kinesis.Client
	auditStream string
}

func NewAuditEmitter(ctx context.Context) (*AuditEmitter, error) {
	awsEndpoint := getAWSEndpoint()
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
		return nil, svcerrors.Internal("audit emit: load aws config", err)
	}

	return &AuditEmitter{
		client:      kinesis.NewFromConfig(cfg),
		auditStream: auditStream,
	}, nil
}

func (e *AuditEmitter) EmitUserAudit(ctx context.Context, operation string, user domain.User) error {
	partitionKey := strconv.Itoa(user.ID)

	auditPayload, err := json.Marshal(UserEventPayload{
		EventType: "audit",
		Entity:    "user",
		Operation: operation,
		User: UserSnapshot{
			ID:          user.ID,
			TenantID:    user.TenantID,
			Username:    user.Username,
			Email:       user.Email,
			PhoneNo:     user.PhoneNo,
			DateOfBirth: user.DateOfBirth,
			Gender:      user.Gender,
			CreatedAt:   user.CreatedAt,
		},
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return svcerrors.Internal("failed to marshal audit event", err)
	}

	if _, err = e.client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(e.auditStream),
		Data:         auditPayload,
		PartitionKey: aws.String(partitionKey),
	}); err != nil {
		return svcerrors.Internal("failed to publish audit event", err)
	}

	logger.Info("audit_emitted", map[string]any{"operation": operation, "user_id": user.ID})
	return nil
}
