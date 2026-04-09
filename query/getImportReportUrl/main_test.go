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

type presignReportSvcMock struct {
	presignFn func(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportReportPresignResult, error)
}

func (m presignReportSvcMock) CreatePendingJobWithPresignedPut(context.Context, string, int) (*service.ImportUploadURLResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m presignReportSvcMock) StartImport(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m presignReportSvcMock) GetJobForTenant(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m presignReportSvcMock) ListJobsForTenant(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m presignReportSvcMock) PresignReportGetURL(ctx context.Context, tenantID string, jobID uuid.UUID) (*service.ImportReportPresignResult, error) {
	if m.presignFn != nil {
		return m.presignFn(ctx, tenantID, jobID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m presignReportSvcMock) ProcessImportJob(context.Context, uuid.UUID) error {
	return svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

type userRepoReportMock struct {
	getWriteByIDFn func(ctx context.Context, id int) (*domain.User, error)
}

func (m userRepoReportMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m userRepoReportMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getWriteByIDFn != nil {
		return m.getWriteByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m userRepoReportMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m userRepoReportMock) DeleteUser(context.Context, int) error { return nil }
func (m userRepoReportMock) GetUser(context.Context, int) (*domain.UserSummary, error) { return nil, nil }
func (m userRepoReportMock) GetUserByEmail(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (m userRepoReportMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoReportMock) CountUsersFiltered(context.Context, domain.ListUsersFilter) (int64, error) {
	return 0, nil
}
func (m userRepoReportMock) RefreshUsersSummaryView(context.Context) error { return nil }
func (m userRepoReportMock) SaveUserReadModel(context.Context, *domain.User) error { return nil }

func bearerReport(t *testing.T, userID int) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-import-report")
	tok, err := auth.GenerateToken(userID, "u@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return "Bearer " + tok
}

func TestHandleRequest_GetImportReportUrl_InvalidJobID(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })
	importSvc = presignReportSvcMock{}
	userRepo = userRepoReportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{Authorization: bearerReport(t, 1), JobID: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportValidation {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestHandleRequest_GetImportReportUrl_Success(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	importSvc = presignReportSvcMock{
		presignFn: func(_ context.Context, tenantID string, jobID uuid.UUID) (*service.ImportReportPresignResult, error) {
			if tenantID != "t1" || jobID != jid {
				t.Fatalf("unexpected %q %v", tenantID, jobID)
			}
			return &service.ImportReportPresignResult{URL: "https://get.example/r", ExpiresInSeconds: 900}, nil
		},
	}
	userRepo = userRepoReportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 3, TenantID: "t1"}, nil
		},
	}

	got, err := HandleRequest(context.Background(), request{
		Authorization: bearerReport(t, 3),
		JobID:         jid.String(),
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got.URL != "https://get.example/r" || got.ExpiresInSeconds != 900 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestHandleRequest_GetImportReportUrl_InvalidState(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	importSvc = presignReportSvcMock{
		presignFn: func(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
			return nil, svcerrors.ImportInvalidState("not completed")
		},
	}
	userRepo = userRepoReportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{
		Authorization: bearerReport(t, 1),
		JobID:         jid.String(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInvalidState {
		t.Fatalf("expected invalid state, got %v", err)
	}
}
