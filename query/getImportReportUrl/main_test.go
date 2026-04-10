package main

import (
	"context"
	"testing"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"
	"project-serverless/internal/service"

	"github.com/google/uuid"
)

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
	importSvc = &helpers.ImportJobService{}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
	importSvc = &helpers.ImportJobService{
		PresignReportGetURLFn: func(_ context.Context, tenantID string, jobID uuid.UUID) (*service.ImportReportPresignResult, error) {
			if tenantID != "t1" || jobID != jid {
				t.Fatalf("unexpected %q %v", tenantID, jobID)
			}
			return &service.ImportReportPresignResult{URL: "https://get.example/r", ExpiresInSeconds: 900}, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
	importSvc = &helpers.ImportJobService{
		PresignReportGetURLFn: func(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
			return nil, svcerrors.ImportInvalidState("not completed")
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
