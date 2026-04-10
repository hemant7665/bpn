package domain

import (
	"time"

	"github.com/google/uuid"
)

// Import job lifecycle statuses (must match DB CHECK on write_model.import_jobs.status).
const (
	ImportStatusPending    = "PENDING"
	ImportStatusAccepted   = "ACCEPTED"
	ImportStatusProcessing = "PROCESSING"
	ImportStatusCompleted  = "COMPLETED"
	ImportStatusFailed     = "FAILED"
)

// ImportJob maps write_model.import_jobs columns; list/get queries use read_model.import_jobs_summary (materialized view, refreshed after writes).
type ImportJob struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey;column:id"`
	TenantID      string     `json:"tenant_id" gorm:"column:tenant_id;not null"`
	RequestedBy   int        `json:"requested_by" gorm:"column:requested_by;not null"`
	Status        string     `json:"status" gorm:"not null"`
	CsvS3Key      string     `json:"csv_s3_key" gorm:"column:csv_s3_key;not null"`
	ReportS3Key   *string    `json:"report_s3_key,omitempty" gorm:"column:report_s3_key"`
	TotalRows     *int       `json:"total_rows,omitempty" gorm:"column:total_rows"`
	PassedRows    *int       `json:"passed_rows,omitempty" gorm:"column:passed_rows"`
	FailedRows    *int       `json:"failed_rows,omitempty" gorm:"column:failed_rows"`
	ErrorMessage  *string    `json:"error_message,omitempty" gorm:"column:error_message"`
	CreatedAt     time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (ImportJob) TableName() string {
	return "import_jobs"
}
