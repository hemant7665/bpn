package orchestrator

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
)

// Service invokes import-related Lambdas.
type Service interface {
	GetImportUploadURL(ctx context.Context) (*UploadURLResult, error)
	StartImport(ctx context.Context, jobID string) (*StartImportResult, error)
	GetImportJob(ctx context.Context, jobID string) (*domain.ImportJob, error)
	ListImportJobs(ctx context.Context, skip, limit int, status *string, createdAtOrder *string) ([]domain.ImportJob, int64, error)
	GetImportReportURL(ctx context.Context, jobID string) (*ReportURLResult, error)
}

type UploadURLResult struct {
	URL              string `json:"url"`
	JobID            string `json:"job_id"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type StartImportResult struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type ReportURLResult struct {
	URL              string `json:"url"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type serviceImpl struct {
	lambdaClient LambdaInvoker
}

func NewService() (Service, error) {
	var cfg aws.Config
	var err error
	ctx := context.TODO()
	environment := os.Getenv("ENVIRONMENT")
	if environment == "local" {
		awsEndpoint := os.Getenv("AWS_LAMBDA_ENDPOINT")
		if awsEndpoint == "" {
			awsEndpoint = os.Getenv("AWS_ENDPOINT_URL")
		}
		if awsEndpoint == "" {
			return nil, svcerrors.Validation("AWS_ENDPOINT_URL or AWS_LAMBDA_ENDPOINT must be set in local environment")
		}
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: awsEndpoint, SigningRegion: "us-east-1"}, nil
				},
			)),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(ctx)
	}
	if err != nil {
		return nil, svcerrors.Internal("service unavailable", err)
	}
	return NewServiceWithClient(lambda.NewFromConfig(cfg)), nil
}

func NewServiceWithClient(c LambdaInvoker) Service {
	return &serviceImpl{lambdaClient: c}
}

func (s *serviceImpl) invoke(ctx context.Context, name string, payload map[string]interface{}) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, svcerrors.Internal("marshal payload", err)
	}
	out, err := s.lambdaClient.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(name),
		Payload:      b,
	})
	if err != nil {
		return nil, svcerrors.Internal("lambda invoke failed", err)
	}
	if out.FunctionError != nil {
		detail := *out.FunctionError
		if len(out.Payload) > 0 {
			detail += ": " + string(out.Payload)
		}
		return nil, svcerrors.Internal("lambda function error: "+detail, nil)
	}
	return out.Payload, nil
}

func mergeAuth(ctx context.Context, payload map[string]interface{}) {
	if hdr := auth.AuthorizationFromContext(ctx); hdr != "" {
		payload["authorization"] = hdr
	}
}

func lambdaName(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

func (s *serviceImpl) GetImportUploadURL(ctx context.Context) (*UploadURLResult, error) {
	payload := map[string]interface{}{}
	mergeAuth(ctx, payload)
	raw, err := s.invoke(ctx, lambdaName("LAMBDA_GET_IMPORT_UPLOAD_URL_NAME", "getImportUploadUrl"), payload)
	if err != nil {
		return nil, err
	}
	var out UploadURLResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, svcerrors.Internal("invalid getImportUploadUrl response", err)
	}
	return &out, nil
}

func (s *serviceImpl) StartImport(ctx context.Context, jobID string) (*StartImportResult, error) {
	payload := map[string]interface{}{"job_id": jobID}
	mergeAuth(ctx, payload)
	raw, err := s.invoke(ctx, lambdaName("LAMBDA_START_IMPORT_NAME", "startImport"), payload)
	if err != nil {
		return nil, err
	}
	var out StartImportResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, svcerrors.Internal("invalid startImport response", err)
	}
	return &out, nil
}

func (s *serviceImpl) GetImportJob(ctx context.Context, jobID string) (*domain.ImportJob, error) {
	payload := map[string]interface{}{"job_id": jobID}
	mergeAuth(ctx, payload)
	raw, err := s.invoke(ctx, lambdaName("LAMBDA_GET_IMPORT_JOB_NAME", "getImportJob"), payload)
	if err != nil {
		return nil, err
	}
	var out domain.ImportJob
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, svcerrors.Internal("invalid getImportJob response", err)
	}
	// JSON uuid unmarshals into uuid.UUID via google/uuid
	return &out, nil
}

func (s *serviceImpl) ListImportJobs(ctx context.Context, skip, limit int, status *string, createdAtOrder *string) ([]domain.ImportJob, int64, error) {
	payload := map[string]interface{}{"skip": skip, "limit": limit}
	if status != nil {
		payload["status"] = *status
	}
	if createdAtOrder != nil && *createdAtOrder != "" {
		payload["created_at_order"] = *createdAtOrder
	}
	mergeAuth(ctx, payload)
	raw, err := s.invoke(ctx, lambdaName("LAMBDA_LIST_IMPORT_JOBS_NAME", "listImportJobs"), payload)
	if err != nil {
		return nil, 0, err
	}
	var out struct {
		Items []domain.ImportJob `json:"items"`
		Total int64              `json:"total"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, 0, svcerrors.Internal("invalid listImportJobs response", err)
	}
	return out.Items, out.Total, nil
}

func (s *serviceImpl) GetImportReportURL(ctx context.Context, jobID string) (*ReportURLResult, error) {
	payload := map[string]interface{}{"job_id": jobID}
	mergeAuth(ctx, payload)
	raw, err := s.invoke(ctx, lambdaName("LAMBDA_GET_IMPORT_REPORT_URL_NAME", "getImportReportUrl"), payload)
	if err != nil {
		return nil, err
	}
	var out ReportURLResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, svcerrors.Internal("invalid getImportReportUrl response", err)
	}
	return &out, nil
}
