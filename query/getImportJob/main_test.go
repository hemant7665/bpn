package main

import (
	"context"
	"testing"
	"time"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"

	"github.com/google/uuid"
)

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
	importSvc = &helpers.ImportJobService{}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
	importSvc = &helpers.ImportJobService{
		GetJobForTenantFn: func(_ context.Context, tenantID string, jobID uuid.UUID) (*domain.ImportJob, error) {
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
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
	importSvc = &helpers.ImportJobService{
		GetJobForTenantFn: func(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
			return nil, svcerrors.ImportJobNotFound("no job")
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
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
