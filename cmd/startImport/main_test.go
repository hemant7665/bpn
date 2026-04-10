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
	importSvc = &helpers.ImportJobService{}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
	importSvc = &helpers.ImportJobService{
		StartImportFn: func(_ context.Context, tenantID string, jobID uuid.UUID) (*service.ImportStartResult, error) {
			if tenantID != "acme" || jobID != jid {
				t.Fatalf("unexpected args: %q %v", tenantID, jobID)
			}
			return &service.ImportStartResult{JobID: jid.String(), Status: domain.ImportStatusAccepted}, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
	if got.JobID != jid.String() || got.Status != domain.ImportStatusAccepted {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestHandleRequest_StartImport_NotFound(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	importSvc = &helpers.ImportJobService{
		StartImportFn: func(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
			return nil, svcerrors.ImportJobNotFound("missing")
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
