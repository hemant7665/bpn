package service

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/s3import"
)

// ImportSQSAPI is the SQS surface used by ImportJobService (*sqs.Client implements it).
type ImportSQSAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// ImportJobAWS groups injectable AWS clients used by ImportJobService.
type ImportJobAWS struct {
	S3Object  s3import.S3ObjectAPI
	S3Presign s3import.S3PresignAPI
	SQS       ImportSQSAPI
}

func s3UsePathStyleForRuntime() bool {
	if os.Getenv("ENVIRONMENT") == "local" {
		return true
	}
	u := strings.ToLower(strings.TrimSpace(os.Getenv("AWS_ENDPOINT_URL")))
	return strings.Contains(u, "localstack") || strings.Contains(u, "localhost:4566") || strings.Contains(u, "127.0.0.1:4566")
}

func s3OptionsForEnv() []func(*s3.Options) {
	if !s3UsePathStyleForRuntime() {
		return nil
	}
	return []func(*s3.Options){
		func(o *s3.Options) { o.UsePathStyle = true },
	}
}

// NewImportJobAWSFromDefaultConfig builds live S3/SQS clients from the default AWS config chain.
func NewImportJobAWSFromDefaultConfig(ctx context.Context) (*ImportJobAWS, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, svcerrors.ImportInternal("aws config", err)
	}
	s3c := s3.NewFromConfig(cfg, s3OptionsForEnv()...)
	return &ImportJobAWS{
		S3Object:  s3c,
		S3Presign: s3.NewPresignClient(s3c),
		SQS:       sqs.NewFromConfig(cfg),
	}, nil
}
