package main

import (
	"context"
	"testing"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/service"

	"github.com/google/uuid"
)

type startImportSvcMock struct {
	startFn func(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportStartResult, error)
}

func (m startImportSvcMock) CreatePendingJobWithPresignedPut(context.Context, string, int) (*service.ImportUploadURLResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m startImportSvcMock) StartImport(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportStartResult, error) {
	if m.startFn != nil {
		return m.startFn(ctx, tenantID, jobID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m startImportSvcMock) GetJobForTenant(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m startImportSvcMock) ListJobsForTenant(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m startImportSvcMock) PresignReportGetURL(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m startImportSvcMock) ProcessImportJob(context.Context, uuid.UUID) error {
	return svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

type userRepoStartImportMock struct {
	getWriteByIDFn func(ctx context.Context, id int) (*domain.User, error)
}

func (m userRepoStartImportMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m userRepoStartImportMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getWriteByIDFn != nil {
		return m.getWriteByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m userRepoStartImportMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m userRepoStartImportMock) DeleteUser(context.Context, int) error { return nil }
func (m userRepoStartImportMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoStartImportMock) GetUserByEmail(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (m userRepoStartImportMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoStartImportMock) CountUsersFiltered(context.Context, domain.ListUsersFilter) (int64, error) {
	return 0, nil
}
func (m userRepoStartImportMock) RefreshUsersSummaryView(context.Context) error { return nil }
func (m userRepoStartImportMock) SaveUserReadModel(context.Context, *domain.User) error { return nil }

func bearerStartImport(t *testing.T, userID int) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-start-import")
	tok, err := auth.GenerateToken(userID, "u@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return "Bearer " + tok
}

func TestHandleRequest_StartImport_InvalidJobID(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })
	importSvc = startImportSvcMock{}
	userRepo = userRepoStartImportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{
		Authorization: bearerStartImport(t, 1),
		JobID:         "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportValidation {
		t.Fatalf("expected import validation, got %v", err)
	}
}

func TestHandleRequest_StartImport_Success(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	importSvc = startImportSvcMock{
		startFn: func(_ context.Context, tenantID string, jobID uuid.UUID) (*service.ImportStartResult, error) {
			if tenantID != "acme" || jobID != jid {
				t.Fatalf("unexpected args: %q %v", tenantID, jobID)
			}
			return &service.ImportStartResult{JobID: jid.String(), Status: domain.ImportStatusPending}, nil
		},
	}
	userRepo = userRepoStartImportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 9, TenantID: "acme"}, nil
		},
	}

	got, err := HandleRequest(context.Background(), request{
		Authorization: bearerStartImport(t, 9),
		JobID:         jid.String(),
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got.JobID != jid.String() || got.Status != domain.ImportStatusPending {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestHandleRequest_StartImport_NotFound(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	importSvc = startImportSvcMock{
		startFn: func(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
			return nil, svcerrors.ImportJobNotFound("missing")
		},
	}
	userRepo = userRepoStartImportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{
		Authorization: bearerStartImport(t, 1),
		JobID:         jid.String(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportJobNotFound {
		t.Fatalf("expected IMPORT_2001, got %v", err)
	}
}
