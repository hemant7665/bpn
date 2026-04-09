package main

import (
	"context"
	"testing"
	"time"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/service"

	"github.com/google/uuid"
)

type getImportJobSvcMock struct {
	getJobFn func(ctx context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error)
}

func (m getImportJobSvcMock) CreatePendingJobWithPresignedPut(context.Context, string, int) (*service.ImportUploadURLResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m getImportJobSvcMock) StartImport(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m getImportJobSvcMock) GetJobForTenant(ctx context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error) {
	if m.getJobFn != nil {
		return m.getJobFn(ctx, tenantID, jobID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m getImportJobSvcMock) ListJobsForTenant(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m getImportJobSvcMock) PresignReportGetURL(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m getImportJobSvcMock) ProcessImportJob(context.Context, uuid.UUID) error {
	return svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

type userRepoGetJobMock struct {
	getWriteByIDFn func(ctx context.Context, id int) (*domain.User, error)
}

func (m userRepoGetJobMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m userRepoGetJobMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getWriteByIDFn != nil {
		return m.getWriteByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m userRepoGetJobMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m userRepoGetJobMock) DeleteUser(context.Context, int) error { return nil }
func (m userRepoGetJobMock) GetUser(context.Context, int) (*domain.UserSummary, error) { return nil, nil }
func (m userRepoGetJobMock) GetUserByEmail(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (m userRepoGetJobMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoGetJobMock) CountUsersFiltered(context.Context, domain.ListUsersFilter) (int64, error) {
	return 0, nil
}
func (m userRepoGetJobMock) RefreshUsersSummaryView(context.Context) error { return nil }
func (m userRepoGetJobMock) SaveUserReadModel(context.Context, *domain.User) error { return nil }

func bearerGetJob(t *testing.T, userID int) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-get-import-job")
	tok, err := auth.GenerateToken(userID, "u@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return "Bearer " + tok
}

func TestHandleRequest_GetImportJob_InvalidJobID(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })
	importSvc = getImportJobSvcMock{}
	userRepo = userRepoGetJobMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{Authorization: bearerGetJob(t, 1), JobID: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportValidation {
		t.Fatalf("expected import validation, got %v", err)
	}
}

func TestHandleRequest_GetImportJob_Success(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	importSvc = getImportJobSvcMock{
		getJobFn: func(_ context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error) {
			if tenantID != "default-tenant" || jobID != jid {
				t.Fatalf("unexpected %q %v", tenantID, jobID)
			}
			return &domain.ImportJob{
				ID:          jid,
				TenantID:    tenantID,
				RequestedBy: 2,
				Status:      domain.ImportStatusCompleted,
				CsvS3Key:    "k",
				CreatedAt:   created,
				UpdatedAt:   created,
			}, nil
		},
	}
	userRepo = userRepoGetJobMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 2, TenantID: "default-tenant"}, nil
		},
	}

	got, err := HandleRequest(context.Background(), request{
		Authorization: bearerGetJob(t, 2),
		JobID:         jid.String(),
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got.ID != jid || got.Status != domain.ImportStatusCompleted {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestHandleRequest_GetImportJob_NotFound(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	importSvc = getImportJobSvcMock{
		getJobFn: func(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
			return nil, svcerrors.ImportJobNotFound("no job")
		},
	}
	userRepo = userRepoGetJobMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{
		Authorization: bearerGetJob(t, 1),
		JobID:         jid.String(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportJobNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
