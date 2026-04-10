package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	"project-serverless/internal/helpers"

	"github.com/google/uuid"
)

func bearerList(t *testing.T, userID int) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-list-import")
	tok, err := auth.GenerateToken(userID, "u@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return "Bearer " + tok
}

func TestHandleRequest_ListImportJobs_Success_NoStatusFilter(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	importSvc = &helpers.ImportJobService{
		ListJobsForTenantFn: func(_ context.Context, tenantID string, skip, limit int, status *string, order string) ([]domain.ImportJob, int64, error) {
			if tenantID != "default-tenant" || skip != 0 || limit != 10 || status != nil || order != "DESC" {
				t.Fatalf("unexpected args: %q skip=%d limit=%d status=%v order=%q", tenantID, skip, limit, status, order)
			}
			return []domain.ImportJob{{ID: jid, TenantID: tenantID, Status: domain.ImportStatusPending}}, 1, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 4, TenantID: "default-tenant"}, nil
		},
	}

	got, err := HandleRequest(context.Background(), request{
		Authorization: bearerList(t, 4),
		Skip:          0,
		Limit:         10,
		Status:        "   ",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != jid {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestHandleRequest_ListImportJobs_Success_WithStatus(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	importSvc = &helpers.ImportJobService{
		ListJobsForTenantFn: func(_ context.Context, tenantID string, skip, limit int, status *string, order string) ([]domain.ImportJob, int64, error) {
			if status == nil || *status != "COMPLETED" || order != "DESC" {
				t.Fatalf("expected status COMPLETED and order DESC, got status=%v order=%q", status, order)
			}
			return nil, 0, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	got, err := HandleRequest(context.Background(), request{
		Authorization: bearerList(t, 1),
		Skip:          5,
		Limit:         20,
		Status:        "COMPLETED",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestHandleRequest_ListImportJobs_OrderAsc(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	importSvc = &helpers.ImportJobService{
		ListJobsForTenantFn: func(_ context.Context, tenantID string, skip, limit int, status *string, order string) ([]domain.ImportJob, int64, error) {
			if order != "ASC" {
				t.Fatalf("expected ASC, got %q", order)
			}
			return nil, 0, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{
		Authorization:  bearerList(t, 1),
		CreatedAtOrder: "asc",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
}

func TestHandleRequest_ListImportJobs_InvalidOrderDefaultsDesc(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	importSvc = &helpers.ImportJobService{
		ListJobsForTenantFn: func(_ context.Context, _ string, _, _ int, _ *string, order string) ([]domain.ImportJob, int64, error) {
			if order != "DESC" {
				t.Fatalf("expected DESC default, got %q", order)
			}
			return nil, 0, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	_, err := HandleRequest(context.Background(), request{
		Authorization:  bearerList(t, 1),
		CreatedAtOrder: "bogus",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
}

func TestHandleRequest_ListImportJobs_JSONOmitsS3Keys(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })

	jid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	rk := "reports/secret.csv"
	importSvc = &helpers.ImportJobService{
		ListJobsForTenantFn: func(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
			return []domain.ImportJob{{
				ID: jid, TenantID: "t", Status: domain.ImportStatusPending,
				CsvS3Key: "tenant/secret/input.csv", ReportS3Key: &rk,
			}}, 1, nil
		},
	}
	userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(context.Context, int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "t"}, nil
		},
	}

	got, err := HandleRequest(context.Background(), request{Authorization: bearerList(t, 1)})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "csv_s3_key") || strings.Contains(s, "report_s3_key") {
		t.Fatalf("response must not expose S3 keys: %s", s)
	}
}

func TestHandleRequest_ListImportJobs_Unauthorized(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })
	importSvc = &helpers.ImportJobService{}
	userRepo = &helpers.UserRepository{}

	_, err := HandleRequest(context.Background(), request{Authorization: ""})
	if err == nil {
		t.Fatal("expected error")
	}
}
