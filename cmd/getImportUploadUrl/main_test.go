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

type importUploadURLSvcMock struct {
	createFn func(ctx context.Context, tenantID string, userID int) (*service.ImportUploadURLResult, error)
}

func (m importUploadURLSvcMock) CreatePendingJobWithPresignedPut(ctx context.Context, tenantID string, userID int) (*service.ImportUploadURLResult, error) {
	if m.createFn != nil {
		return m.createFn(ctx, tenantID, userID)
	}
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

func (m importUploadURLSvcMock) StartImport(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m importUploadURLSvcMock) GetJobForTenant(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m importUploadURLSvcMock) ListJobsForTenant(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m importUploadURLSvcMock) PresignReportGetURL(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m importUploadURLSvcMock) ProcessImportJob(context.Context, uuid.UUID) error {
	return svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

type userRepoForImportMock struct {
	getWriteByIDFn func(ctx context.Context, id int) (*domain.User, error)
}

func (m userRepoForImportMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m userRepoForImportMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getWriteByIDFn != nil {
		return m.getWriteByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m userRepoForImportMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m userRepoForImportMock) DeleteUser(context.Context, int) error { return nil }
func (m userRepoForImportMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoForImportMock) GetUserByEmail(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (m userRepoForImportMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoForImportMock) CountUsersFiltered(context.Context, domain.ListUsersFilter) (int64, error) {
	return 0, nil
}
func (m userRepoForImportMock) RefreshUsersSummaryView(context.Context) error { return nil }
func (m userRepoForImportMock) SaveUserReadModel(context.Context, *domain.User) error { return nil }

func bearerForUser(t *testing.T, userID int) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-import-upload")
	tok, err := auth.GenerateToken(userID, "tester@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return "Bearer " + tok
}

func TestHandleRequest_GetImportUploadUrl_Unauthorized(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	importSvc = importUploadURLSvcMock{}
	userRepo = userRepoForImportMock{}

	_, err := HandleRequest(context.Background(), request{Authorization: ""})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized AppError, got %v", err)
	}
}

func TestHandleRequest_GetImportUploadUrl_Success(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	importSvc = importUploadURLSvcMock{
		createFn: func(_ context.Context, tenantID string, userID int) (*service.ImportUploadURLResult, error) {
			if tenantID != "default-tenant" || userID != 5 {
				t.Fatalf("unexpected tenant/user: %q %d", tenantID, userID)
			}
			return &service.ImportUploadURLResult{
				URL:              "https://s3.example/presigned",
				JobID:            "11111111-1111-1111-1111-111111111111",
				CsvS3Key:         "default-tenant/11111111-1111-1111-1111-111111111111/input.csv",
				ExpiresInSeconds: 900,
			}, nil
		},
	}
	userRepo = userRepoForImportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 5, TenantID: "default-tenant"}, nil
		},
	}

	authz := bearerForUser(t, 5)
	got, err := HandleRequest(context.Background(), request{Authorization: authz})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got.URL != "https://s3.example/presigned" || got.JobID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected response: %+v", got)
	}
	if got.ExpiresInSeconds != 900 {
		t.Fatalf("expires: %d", got.ExpiresInSeconds)
	}
}

func TestHandleRequest_GetImportUploadUrl_ServiceError(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	importSvc = importUploadURLSvcMock{
		createFn: func(context.Context, string, int) (*service.ImportUploadURLResult, error) {
			return nil, svcerrors.ImportInternal("bucket missing", nil)
		},
	}
	userRepo = userRepoForImportMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{Authorization: bearerForUser(t, 1)})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInternal {
		t.Fatalf("expected import internal error, got %v", err)
	}
}
