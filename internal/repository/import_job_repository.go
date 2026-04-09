package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"project-serverless/internal/domain"
)

// importJobsReadView is read_model.import_jobs_summary (materialized view; migrate 000003).
const importJobsReadView = "read_model.import_jobs_summary"

type ImportJobRepository interface {
	Create(ctx context.Context, job *domain.ImportJob) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error)
	GetByIDAndTenant(ctx context.Context, id uuid.UUID, tenantID string) (*domain.ImportJob, error)
	ListByTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error)
	ClaimPending(ctx context.Context, id uuid.UUID) (bool, error)
	MarkFailed(ctx context.Context, id uuid.UUID, message string) error
	MarkCompleted(ctx context.Context, id uuid.UUID, reportS3Key string, total, passed, failed int) error
}

type importJobRepositoryImpl struct {
	db *gorm.DB
}

func NewImportJobRepository(db *gorm.DB) ImportJobRepository {
	return &importJobRepositoryImpl{db: db}
}

func (r *importJobRepositoryImpl) refreshImportJobsSummary(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	if _, err := sqlDB.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY read_model.import_jobs_summary"); err != nil {
		_, err2 := sqlDB.ExecContext(ctx, "REFRESH MATERIALIZED VIEW read_model.import_jobs_summary")
		return err2
	}
	return nil
}

func (r *importJobRepositoryImpl) Create(ctx context.Context, job *domain.ImportJob) error {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		return err
	}
	return r.refreshImportJobsSummary(ctx)
}

func (r *importJobRepositoryImpl) GetByIDAndTenant(ctx context.Context, id uuid.UUID, tenantID string) (*domain.ImportJob, error) {
	var j domain.ImportJob
	err := r.db.WithContext(ctx).Table(importJobsReadView).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&j).Error
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *importJobRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*domain.ImportJob, error) {
	var j domain.ImportJob
	err := r.db.WithContext(ctx).Table(importJobsReadView).Where("id = ?", id).First(&j).Error
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// ListByTenant returns jobs ordered by created_at (ASC or DESC; invalid values default to DESC).
func (r *importJobRepositoryImpl) ListByTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error) {
	order := strings.ToUpper(strings.TrimSpace(createdAtOrder))
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	q := r.db.WithContext(ctx).Table(importJobsReadView).Where("tenant_id = ?", tenantID)
	if status != nil && *status != "" {
		q = q.Where("status = ?", *status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []domain.ImportJob
	err := q.Order("created_at " + order).Offset(skip).Limit(limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *importJobRepositoryImpl) ClaimPending(ctx context.Context, id uuid.UUID) (bool, error) {
	res := r.db.WithContext(ctx).Model(&domain.ImportJob{}).
		Where("id = ? AND status = ?", id, domain.ImportStatusPending).
		Updates(map[string]interface{}{
			"status":     domain.ImportStatusProcessing,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected != 1 {
		return false, nil
	}
	if err := r.refreshImportJobsSummary(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *importJobRepositoryImpl) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	if err := r.db.WithContext(ctx).Model(&domain.ImportJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        domain.ImportStatusFailed,
		"error_message": message,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return err
	}
	return r.refreshImportJobsSummary(ctx)
}

func (r *importJobRepositoryImpl) MarkCompleted(ctx context.Context, id uuid.UUID, reportS3Key string, total, passed, failed int) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&domain.ImportJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        domain.ImportStatusCompleted,
		"report_s3_key": reportS3Key,
		"total_rows":    total,
		"passed_rows":   passed,
		"failed_rows":   failed,
		"error_message": nil,
		"updated_at":    now,
	}).Error; err != nil {
		return err
	}
	return r.refreshImportJobsSummary(ctx)
}
