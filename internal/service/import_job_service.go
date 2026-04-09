package service

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/s3import"
	"project-serverless/modules/csvimport"
)

// loadImportAWSConfig loads the default AWS SDK config used by import S3/SQS paths.
func loadImportAWSConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, svcerrors.ImportInternal("aws config", err)
	}
	return cfg, nil
}

// importS3BucketName returns IMPORT_S3_BUCKET trimmed (may be empty).
func importS3BucketName() string {
	return strings.TrimSpace(os.Getenv("IMPORT_S3_BUCKET"))
}

// importS3BucketFromEnv returns the import bucket or an error if unset (API-style handlers).
func importS3BucketFromEnv() (string, error) {
	bucket := importS3BucketName()
	if bucket == "" {
		return "", svcerrors.ImportInternal("IMPORT_S3_BUCKET is not set", nil)
	}
	return bucket, nil
}

// markFailedImportS3BucketUnset records the standard message when the bucket env is missing (worker path).
func markFailedImportS3BucketUnset(ctx context.Context, repo repository.ImportJobRepository, jobID uuid.UUID) {
	_ = repo.MarkFailed(ctx, jobID, "IMPORT_S3_BUCKET is not set")
}

// markFailedAWSConfig records load failure for the worker (preserves underlying error text when available).
func markFailedAWSConfig(ctx context.Context, repo repository.ImportJobRepository, jobID uuid.UUID, err error) {
	msg := "aws config"
	if ce, ok := svcerrors.IsAppError(err); ok && ce.Cause != nil {
		msg = msg + ": " + ce.Cause.Error()
	} else if err != nil {
		msg = msg + ": " + err.Error()
	}
	_ = repo.MarkFailed(ctx, jobID, msg)
}

type importJobServiceImpl struct {
	repo repository.ImportJobRepository
}

// NewImportJobService builds the import application service (Lambda handlers should depend on this, not the repository).
func NewImportJobService(repo repository.ImportJobRepository) ImportJobService {
	return &importJobServiceImpl{repo: repo}
}

// s3OptionsForEnv returns path-style addressing for LocalStack. Virtual-hosted presigned URLs
// can make LocalStack treat the first key segment as the bucket ("imports"), breaking PUT/GET from the host.
func s3OptionsForEnv() []func(*s3.Options) {
	if !s3UsePathStyleForRuntime() {
		return nil
	}
	return []func(*s3.Options){
		func(o *s3.Options) { o.UsePathStyle = true },
	}
}

func s3UsePathStyleForRuntime() bool {
	if os.Getenv("ENVIRONMENT") == "local" {
		return true
	}
	u := strings.ToLower(strings.TrimSpace(os.Getenv("AWS_ENDPOINT_URL")))
	return strings.Contains(u, "localstack") || strings.Contains(u, "localhost:4566") || strings.Contains(u, "127.0.0.1:4566")
}

func (s *importJobServiceImpl) CreatePendingJobWithPresignedPut(ctx context.Context, tenantID string, userID int) (*ImportUploadURLResult, error) {
	bucket, err := importS3BucketFromEnv()
	if err != nil {
		return nil, err
	}

	jobID := uuid.New()
	csvKey, _ := s3import.ObjectKeys(tenantID, jobID)
	job := &domain.ImportJob{
		ID:          jobID,
		TenantID:    tenantID,
		RequestedBy: userID,
		Status:      domain.ImportStatusPending,
		CsvS3Key:    csvKey,
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, svcerrors.ImportInternal("failed to create import job", err)
	}

	awsCfg, err := loadImportAWSConfig(ctx)
	if err != nil {
		return nil, err
	}
	presigner := s3.NewPresignClient(s3.NewFromConfig(awsCfg, s3OptionsForEnv()...))
	expiry := 15 * time.Minute
	urlStr, expSec, err := s3import.PresignPut(ctx, presigner, bucket, csvKey, expiry)
	if err != nil {
		return nil, svcerrors.ImportInternal("presign put failed", err)
	}
	return &ImportUploadURLResult{
		URL:              urlStr,
		JobID:            jobID.String(),
		CsvS3Key:         csvKey,
		ExpiresInSeconds: expSec,
	}, nil
}

func (s *importJobServiceImpl) StartImport(ctx context.Context, tenantID string, jobID uuid.UUID) (*ImportStartResult, error) {
	job, err := s.repo.GetByIDAndTenant(ctx, jobID, tenantID)
	if err != nil {
		return nil, svcerrors.ImportJobNotFound("import job not found")
	}
	if job.Status != domain.ImportStatusPending {
		return nil, svcerrors.ImportInvalidState("import cannot be started: job is not PENDING")
	}

	bucket, err := importS3BucketFromEnv()
	if err != nil {
		return nil, err
	}

	awsCfg, err := loadImportAWSConfig(ctx)
	if err != nil {
		return nil, err
	}
	s3c := s3.NewFromConfig(awsCfg, s3OptionsForEnv()...)
	if err := s3import.HeadObjectExists(ctx, s3c, bucket, job.CsvS3Key); err != nil {
		return nil, svcerrors.ImportS3Missing("csv file not found in storage; upload before startImport")
	}

	queueURL := strings.TrimSpace(os.Getenv("IMPORT_QUEUE_URL"))
	if queueURL == "" {
		return nil, svcerrors.ImportInternal("IMPORT_QUEUE_URL is not set", nil)
	}

	body, _ := json.Marshal(map[string]string{"job_id": jobID.String()})
	sqsClient := sqs.NewFromConfig(awsCfg)
	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return nil, svcerrors.ImportQueue("failed to enqueue import job", err)
	}
	return &ImportStartResult{JobID: jobID.String(), Status: job.Status}, nil
}

func (s *importJobServiceImpl) GetJobForTenant(ctx context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error) {
	job, err := s.repo.GetByIDAndTenant(ctx, jobID, tenantID)
	if err != nil {
		return nil, svcerrors.ImportJobNotFound("import job not found")
	}
	return job, nil
}

func (s *importJobServiceImpl) ListJobsForTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error) {
	if skip < 0 {
		skip = 0
	}
	if limit <= 0 {
		limit = 0
	}
	if limit > 100 {
		limit = 100
	}
	order := strings.ToUpper(strings.TrimSpace(createdAtOrder))
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	items, total, err := s.repo.ListByTenant(ctx, tenantID, skip, limit, status, order)
	if err != nil {
		return nil, 0, svcerrors.ImportInternal("list import jobs failed", err)
	}
	return items, total, nil
}

func (s *importJobServiceImpl) PresignReportGetURL(ctx context.Context, tenantID string, jobID uuid.UUID) (*ImportReportPresignResult, error) {
	job, err := s.repo.GetByIDAndTenant(ctx, jobID, tenantID)
	if err != nil {
		return nil, svcerrors.ImportJobNotFound("import job not found")
	}
	if job.Status != domain.ImportStatusCompleted || job.ReportS3Key == nil || strings.TrimSpace(*job.ReportS3Key) == "" {
		return nil, svcerrors.ImportInvalidState("report is only available when job status is COMPLETED")
	}

	bucket, err := importS3BucketFromEnv()
	if err != nil {
		return nil, err
	}

	awsCfg, err := loadImportAWSConfig(ctx)
	if err != nil {
		return nil, err
	}
	presigner := s3.NewPresignClient(s3.NewFromConfig(awsCfg, s3OptionsForEnv()...))
	expiry := 900 * time.Second
	urlStr, expSec, err := s3import.PresignGet(ctx, presigner, bucket, strings.TrimSpace(*job.ReportS3Key), expiry)
	if err != nil {
		return nil, svcerrors.ImportInternal("presign get failed", err)
	}
	return &ImportReportPresignResult{URL: urlStr, ExpiresInSeconds: expSec}, nil
}

func (s *importJobServiceImpl) ProcessImportJob(ctx context.Context, jobID uuid.UUID) error {
	claimed, err := s.repo.ClaimPending(ctx, jobID)
	if err != nil {
		return err
	}
	if !claimed {
		j2, e := s.repo.GetByID(ctx, jobID)
		if e == nil && j2 != nil {
			switch j2.Status {
			case domain.ImportStatusCompleted, domain.ImportStatusFailed, domain.ImportStatusProcessing:
				return nil
			}
		}
		return nil
	}

	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		logger.Error("import_worker_job_missing_after_claim", map[string]any{"job_id": jobID.String(), "error": err.Error()})
		return nil
	}

	bucket := importS3BucketName()
	if bucket == "" {
		markFailedImportS3BucketUnset(ctx, s.repo, jobID)
		return nil
	}

	awsCfg, err := loadImportAWSConfig(ctx)
	if err != nil {
		markFailedAWSConfig(ctx, s.repo, jobID, err)
		return nil
	}
	s3c := s3.NewFromConfig(awsCfg, s3OptionsForEnv()...)

	out, err := s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(job.CsvS3Key),
	})
	if err != nil {
		_ = s.repo.MarkFailed(ctx, jobID, "failed to read csv from s3: "+err.Error())
		return nil
	}
	defer out.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(out.Body); err != nil {
		_ = s.repo.MarkFailed(ctx, jobID, "failed to read csv body: "+err.Error())
		return nil
	}

	rep, err := csvimport.ParseAndValidateCSV(buf)
	if err != nil {
		_ = s.repo.MarkFailed(ctx, jobID, err.Error())
		return nil
	}

	reportKey := reportS3KeyForJob(job)
	payload, err := json.Marshal(rep)
	if err != nil {
		_ = s.repo.MarkFailed(ctx, jobID, "failed to marshal report: "+err.Error())
		return nil
	}

	_, err = s3c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(reportKey),
		Body:        bytes.NewReader(payload),
		ContentType: aws.String(s3import.ReportContentType),
	})
	if err != nil {
		_ = s.repo.MarkFailed(ctx, jobID, "failed to upload report: "+err.Error())
		return nil
	}

	return s.repo.MarkCompleted(ctx, jobID, reportKey, rep.Total, rep.Passed, rep.Failed)
}

func reportS3KeyForJob(job *domain.ImportJob) string {
	_, rk := s3import.ObjectKeys(job.TenantID, job.ID)
	return rk
}
