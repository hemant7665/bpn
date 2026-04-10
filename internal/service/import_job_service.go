package service

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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
	aws  *ImportJobAWS
}

// NewImportJobService builds the import application service. Pass aws from NewImportJobAWSFromDefaultConfig in production; nil aws is allowed for tests that only exercise DB-only paths.
func NewImportJobService(repo repository.ImportJobRepository, aws *ImportJobAWS) ImportJobService {
	return &importJobServiceImpl{repo: repo, aws: aws}
}

func (s *importJobServiceImpl) requireS3Presign() (s3import.S3PresignAPI, error) {
	if s.aws == nil || s.aws.S3Presign == nil {
		return nil, svcerrors.ImportInternal("import S3 presigner not configured", nil)
	}
	return s.aws.S3Presign, nil
}

func (s *importJobServiceImpl) requireS3Object() (s3import.S3ObjectAPI, error) {
	if s.aws == nil || s.aws.S3Object == nil {
		return nil, svcerrors.ImportInternal("import S3 API client not configured", nil)
	}
	return s.aws.S3Object, nil
}

func (s *importJobServiceImpl) requireSQS() (ImportSQSAPI, error) {
	if s.aws == nil || s.aws.SQS == nil {
		return nil, svcerrors.ImportInternal("import SQS client not configured", nil)
	}
	return s.aws.SQS, nil
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

	presigner, err := s.requireS3Presign()
	if err != nil {
		return nil, err
	}
	expiry := 15 * time.Minute
	urlStr, expSec, err := s3import.PresignPut(ctx, presigner, bucket, csvKey, expiry)
	if err != nil {
		return nil, svcerrors.ImportInternal("presign put failed", err)
	}
	return &ImportUploadURLResult{
		URL:              urlStr,
		JobID:            jobID.String(),
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

	s3c, err := s.requireS3Object()
	if err != nil {
		return nil, err
	}
	if err := s3import.HeadObjectExists(ctx, s3c, bucket, job.CsvS3Key); err != nil {
		return nil, svcerrors.ImportS3Missing("csv file not found in storage; upload before startImport")
	}

	queueURL := strings.TrimSpace(os.Getenv("IMPORT_QUEUE_URL"))
	if queueURL == "" {
		return nil, svcerrors.ImportInternal("IMPORT_QUEUE_URL is not set", nil)
	}

	sqsClient, err := s.requireSQS()
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]string{"job_id": jobID.String()})
	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return nil, svcerrors.ImportQueue("failed to enqueue import job", err)
	}
	if err := s.repo.MarkAccepted(ctx, jobID); err != nil {
		return nil, svcerrors.ImportInternal("failed to mark import job accepted", err)
	}
	return &ImportStartResult{JobID: jobID.String(), Status: domain.ImportStatusAccepted}, nil
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
		limit = 20
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

	presigner, err := s.requireS3Presign()
	if err != nil {
		return nil, err
	}
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

	s3c, err := s.requireS3Object()
	if err != nil {
		markFailedAWSConfig(ctx, s.repo, jobID, err)
		return nil
	}

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
