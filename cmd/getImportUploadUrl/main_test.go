package main

import (
	"context"
	"testing"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"
	"project-serverless/internal/service"
)

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

	importSvc = &helpers.ImportJobService{}
	userRepo = &helpers.UserRepository{}

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

	importSvc = &helpers.ImportJobService{
		CreatePendingJobWithPresignedPutFn: func(_ context.Context, tenantID string, userID int) (*service.ImportUploadURLResult, error) {
			if tenantID != "default-tenant" || userID != 5 {
				t.Fatalf("unexpected tenant/user: %q %d", tenantID, userID)
			}
			return &service.ImportUploadURLResult{
				URL:              "https://s3.example/presigned",
				JobID:            "11111111-1111-1111-1111-111111111111",
				ExpiresInSeconds: 900,
			}, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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

	importSvc = &helpers.ImportJobService{
		CreatePendingJobWithPresignedPutFn: func(context.Context, string, int) (*service.ImportUploadURLResult, error) {
			return nil, svcerrors.ImportInternal("bucket missing", nil)
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
