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

type listImportJobsSvcMock struct {
	listFn func(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error)
}

func (m listImportJobsSvcMock) CreatePendingJobWithPresignedPut(context.Context, string, int) (*service.ImportUploadURLResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m listImportJobsSvcMock) StartImport(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m listImportJobsSvcMock) GetJobForTenant(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m listImportJobsSvcMock) ListJobsForTenant(ctx context.Context, tenantID string, skip, limit int, status *string, createdAtOrder string) ([]domain.ImportJob, int64, error) {
	if m.listFn != nil {
		return m.listFn(ctx, tenantID, skip, limit, status, createdAtOrder)
	}
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m listImportJobsSvcMock) PresignReportGetURL(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m listImportJobsSvcMock) ProcessImportJob(context.Context, uuid.UUID) error {
	return svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}

type userRepoListMock struct {
	getWriteByIDFn func(ctx context.Context, id int) (*domain.User, error)
}

func (m userRepoListMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m userRepoListMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getWriteByIDFn != nil {
		return m.getWriteByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m userRepoListMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m userRepoListMock) DeleteUser(context.Context, int) error { return nil }
func (m userRepoListMock) GetUser(context.Context, int) (*domain.UserSummary, error) { return nil, nil }
func (m userRepoListMock) GetUserByEmail(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (m userRepoListMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
	return nil, nil
}
func (m userRepoListMock) CountUsersFiltered(context.Context, domain.ListUsersFilter) (int64, error) {
	return 0, nil
}
func (m userRepoListMock) RefreshUsersSummaryView(context.Context) error { return nil }
func (m userRepoListMock) SaveUserReadModel(context.Context, *domain.User) error { return nil }

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
	importSvc = listImportJobsSvcMock{
		listFn: func(_ context.Context, tenantID string, skip, limit int, status *string, order string) ([]domain.ImportJob, int64, error) {
			if tenantID != "default-tenant" || skip != 0 || limit != 10 || status != nil || order != "DESC" {
				t.Fatalf("unexpected args: %q skip=%d limit=%d status=%v order=%q", tenantID, skip, limit, status, order)
			}
			return []domain.ImportJob{{ID: jid, TenantID: tenantID, Status: domain.ImportStatusPending}}, 1, nil
		},
	}
	userRepo = userRepoListMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
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

	importSvc = listImportJobsSvcMock{
		listFn: func(_ context.Context, tenantID string, skip, limit int, status *string, order string) ([]domain.ImportJob, int64, error) {
			if status == nil || *status != "COMPLETED" || order != "DESC" {
				t.Fatalf("expected status COMPLETED and order DESC, got status=%v order=%q", status, order)
			}
			return nil, 0, nil
		},
	}
	userRepo = userRepoListMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
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

	importSvc = listImportJobsSvcMock{
		listFn: func(_ context.Context, tenantID string, skip, limit int, status *string, order string) ([]domain.ImportJob, int64, error) {
			if order != "ASC" {
				t.Fatalf("expected ASC, got %q", order)
			}
			return nil, 0, nil
		},
	}
	userRepo = userRepoListMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
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

	importSvc = listImportJobsSvcMock{
		listFn: func(_ context.Context, _ string, _, _ int, _ *string, order string) ([]domain.ImportJob, int64, error) {
			if order != "DESC" {
				t.Fatalf("expected DESC default, got %q", order)
			}
			return nil, 0, nil
		},
	}
	userRepo = userRepoListMock{
		getWriteByIDFn: func(context.Context, int) (*domain.User, error) {
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

func TestHandleRequest_ListImportJobs_Unauthorized(t *testing.T) {
	oldSvc, oldRepo := importSvc, userRepo
	t.Cleanup(func() { importSvc, userRepo = oldSvc, oldRepo })
	importSvc = listImportJobsSvcMock{}
	userRepo = userRepoListMock{}

	_, err := HandleRequest(context.Background(), request{Authorization: ""})
	if err == nil {
		t.Fatal("expected error")
	}
}
