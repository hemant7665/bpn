package helpers

import (
	"context"

	"github.com/google/uuid"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

// ImportJobListCapture records arguments from the last ListByTenant call when non-nil.
type ImportJobListCapture struct {
	TenantID string
	Skip     int
	Limit    int
	Order    string
}

// ImportJobRepository is a configurable test double implementing repository.ImportJobRepository.
type ImportJobRepository struct {
	CreateFn       func(ctx context.Context, job *domain.ImportJob) error
	GetByIDFn      func(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error)
	GetByTenantFn  func(ctx context.Context, id uuid.UUID, tenantID string) (*domain.ImportJob, error)
	ListByTenantFn func(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error)
	ListCapture    *ImportJobListCapture

	ClaimFn         func(ctx context.Context, id uuid.UUID) (bool, error)
	MarkAcceptedFn  func(ctx context.Context, id uuid.UUID) error
	MarkFailedFn    func(ctx context.Context, id uuid.UUID, message string) error
	MarkFailedMsgs  []string
	MarkCompletedFn func(ctx context.Context, id uuid.UUID, reportS3Key string, total, passed, failed int) error
}

var _ repository.ImportJobRepository = (*ImportJobRepository)(nil)

func (m *ImportJobRepository) Create(ctx context.Context, job *domain.ImportJob) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, job)
	}
	return nil
}

func (m *ImportJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *ImportJobRepository) GetByIDAndTenant(ctx context.Context, id uuid.UUID, tenantID string) (*domain.ImportJob, error) {
	if m.GetByTenantFn != nil {
		return m.GetByTenantFn(ctx, id, tenantID)
	}
	return nil, nil
}

func (m *ImportJobRepository) ListByTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error) {
	if m.ListCapture != nil {
		m.ListCapture.TenantID = tenantID
		m.ListCapture.Skip = skip
		m.ListCapture.Limit = limit
		m.ListCapture.Order = createdAtOrder
	}
	if m.ListByTenantFn != nil {
		return m.ListByTenantFn(ctx, tenantID, skip, limit, status, createdAtOrder)
	}
	return nil, 0, nil
}

func (m *ImportJobRepository) ClaimPending(ctx context.Context, id uuid.UUID) (bool, error) {
	if m.ClaimFn != nil {
		return m.ClaimFn(ctx, id)
	}
	return false, nil
}

func (m *ImportJobRepository) MarkAccepted(ctx context.Context, id uuid.UUID) error {
	if m.MarkAcceptedFn != nil {
		return m.MarkAcceptedFn(ctx, id)
	}
	return nil
}

func (m *ImportJobRepository) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	m.MarkFailedMsgs = append(m.MarkFailedMsgs, message)
	if m.MarkFailedFn != nil {
		return m.MarkFailedFn(ctx, id, message)
	}
	return nil
}

func (m *ImportJobRepository) MarkCompleted(ctx context.Context, id uuid.UUID, reportS3Key string, total, passed, failed int) error {
	if m.MarkCompletedFn != nil {
		return m.MarkCompletedFn(ctx, id, reportS3Key, total, passed, failed)
	}
	return nil
}

// ImportJobService is a configurable test double implementing service.ImportJobService.
type ImportJobService struct {
	CreatePendingJobWithPresignedPutFn func(ctx context.Context, tenantID string, userID int) (*service.ImportUploadURLResult, error)
	StartImportFn                      func(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportStartResult, error)
	GetJobForTenantFn                  func(ctx context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error)
	ListJobsForTenantFn                func(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error)
	PresignReportGetURLFn              func(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportReportPresignResult, error)
	ProcessImportJobFn                 func(ctx context.Context, jobID uuid.UUID) error
}

var _ service.ImportJobService = (*ImportJobService)(nil)

func (m *ImportJobService) CreatePendingJobWithPresignedPut(ctx context.Context, tenantID string, userID int) (*service.ImportUploadURLResult, error) {
	if m.CreatePendingJobWithPresignedPutFn != nil {
		return m.CreatePendingJobWithPresignedPutFn(ctx, tenantID, userID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

func (m *ImportJobService) StartImport(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportStartResult, error) {
	if m.StartImportFn != nil {
		return m.StartImportFn(ctx, tenantID, jobID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

func (m *ImportJobService) GetJobForTenant(ctx context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error) {
	if m.GetJobForTenantFn != nil {
		return m.GetJobForTenantFn(ctx, tenantID, jobID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

func (m *ImportJobService) ListJobsForTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error) {
	if m.ListJobsForTenantFn != nil {
		return m.ListJobsForTenantFn(ctx, tenantID, skip, limit, status, createdAtOrder)
	}
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

func (m *ImportJobService) PresignReportGetURL(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportReportPresignResult, error) {
	if m.PresignReportGetURLFn != nil {
		return m.PresignReportGetURLFn(ctx, tenantID, jobID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

func (m *ImportJobService) ProcessImportJob(ctx context.Context, jobID uuid.UUID) error {
	if m.ProcessImportJobFn != nil {
		return m.ProcessImportJobFn(ctx, jobID)
	}
	return svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
