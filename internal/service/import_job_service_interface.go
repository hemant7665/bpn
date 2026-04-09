package service

import (
	"context"

	"github.com/google/uuid"

	"project-serverless/internal/domain"
)

// ImportJobService is the application service for CSV bulk import jobs (command/query Lambdas + worker).
// Used by cmd/getImportUploadUrl, startImport, query/getImportJob, listImportJobs, getImportReportUrl, workers/importJobWorker.
type ImportJobService interface {
	CreatePendingJobWithPresignedPut(ctx context.Context, tenantID string, userID int) (*ImportUploadURLResult, error)
	StartImport(ctx context.Context, tenantID string, jobID uuid.UUID) (*ImportStartResult, error)
	GetJobForTenant(ctx context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error)
	ListJobsForTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error)
	PresignReportGetURL(ctx context.Context, tenantID string, jobID uuid.UUID) (*ImportReportPresignResult, error)
	ProcessImportJob(ctx context.Context, jobID uuid.UUID) error
}

// ImportUploadURLResult is the application output for a new PENDING job + presigned PUT.
type ImportUploadURLResult struct {
	URL              string
	JobID            string
	CsvS3Key         string
	ExpiresInSeconds int
}

type ImportStartResult struct {
	JobID  string
	Status string
}

type ImportReportPresignResult struct {
	URL              string
	ExpiresInSeconds int
}
