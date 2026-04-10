package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"
	"project-serverless/internal/service"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestListJobsForTenant_normalizesSkipLimitOrder(t *testing.T) {
	capture := &helpers.ImportJobListCapture{}
	repo := &helpers.ImportJobRepository{
		ListCapture: capture,
		ListByTenantFn: func(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
			return nil, 0, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	ctx := context.Background()

	_, _, err := svc.ListJobsForTenant(ctx, "t1", -5, 0, nil, "bogus")
	if err != nil {
		t.Fatal(err)
	}
	if capture.Skip != 0 || capture.Limit != 20 || capture.Order != "DESC" {
		t.Fatalf("capture %+v", capture)
	}

	_, _, err = svc.ListJobsForTenant(ctx, "t1", 0, 500, nil, "asc")
	if err != nil {
		t.Fatal(err)
	}
	if capture.Limit != 100 || capture.Order != "ASC" {
		t.Fatalf("second capture skip=%d limit=%d order=%q", capture.Skip, capture.Limit, capture.Order)
	}
}

func TestListJobsForTenant_repoError(t *testing.T) {
	want := errors.New("db")
	repo := &helpers.ImportJobRepository{
		ListByTenantFn: func(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
			return nil, 0, want
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, _, err := svc.ListJobsForTenant(context.Background(), "t", 0, 10, nil, "DESC")
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInternal {
		t.Fatalf("expected import internal, got %v", err)
	}
}

func TestGetJobForTenant_notFound(t *testing.T) {
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc := service.NewImportJobService(repo, nil)
	jid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	_, err := svc.GetJobForTenant(context.Background(), "t", jid)
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportJobNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestGetJobForTenant_success(t *testing.T) {
	want := &domain.ImportJob{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), TenantID: "acme", Status: domain.ImportStatusPending}
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return want, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	got, err := svc.GetJobForTenant(context.Background(), "acme", want.ID)
	if err != nil || got != want {
		t.Fatalf("got (%v, %v)", got, err)
	}
}

func TestStartImport_notFound(t *testing.T) {
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, err := svc.StartImport(context.Background(), "t", uuid.New())
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportJobNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestStartImport_notPending(t *testing.T) {
	j := &domain.ImportJob{Status: domain.ImportStatusAccepted}
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return j, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, err := svc.StartImport(context.Background(), "t", uuid.New())
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInvalidState {
		t.Fatalf("got %v", err)
	}
}

func TestStartImport_missingImportBucket(t *testing.T) {
	t.Setenv("IMPORT_S3_BUCKET", "")
	j := &domain.ImportJob{Status: domain.ImportStatusPending, CsvS3Key: "k"}
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return j, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, err := svc.StartImport(context.Background(), "t", uuid.New())
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInternal || !strings.Contains(ae.Message, "IMPORT_S3_BUCKET") {
		t.Fatalf("got %v", err)
	}
}

func TestCreatePendingJobWithPresignedPut_missingBucket(t *testing.T) {
	t.Setenv("IMPORT_S3_BUCKET", "")
	svc := service.NewImportJobService(&helpers.ImportJobRepository{}, nil)
	_, err := svc.CreatePendingJobWithPresignedPut(context.Background(), "t", 1)
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInternal {
		t.Fatalf("got %v", err)
	}
}

func TestPresignReportGetURL_notCompleted(t *testing.T) {
	t.Setenv("IMPORT_S3_BUCKET", "b")
	j := &domain.ImportJob{Status: domain.ImportStatusPending}
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return j, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, err := svc.PresignReportGetURL(context.Background(), "t", uuid.New())
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInvalidState {
		t.Fatalf("got %v", err)
	}
}

func TestPresignReportGetURL_completedNoReportKey(t *testing.T) {
	t.Setenv("IMPORT_S3_BUCKET", "b")
	empty := ""
	j := &domain.ImportJob{Status: domain.ImportStatusCompleted, ReportS3Key: &empty}
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return j, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, err := svc.PresignReportGetURL(context.Background(), "t", uuid.New())
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInvalidState {
		t.Fatalf("got %v", err)
	}
}

func TestPresignReportGetURL_missingBucket(t *testing.T) {
	t.Setenv("IMPORT_S3_BUCKET", "")
	k := "reports/x"
	j := &domain.ImportJob{Status: domain.ImportStatusCompleted, ReportS3Key: &k}
	repo := &helpers.ImportJobRepository{
		GetByTenantFn: func(context.Context, uuid.UUID, string) (*domain.ImportJob, error) {
			return j, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	_, err := svc.PresignReportGetURL(context.Background(), "t", uuid.New())
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeImportInternal {
		t.Fatalf("got %v", err)
	}
}

func TestProcessImportJob_claimErrorPropagates(t *testing.T) {
	claimErr := errors.New("claim tx failed")
	repo := &helpers.ImportJobRepository{
		ClaimFn: func(context.Context, uuid.UUID) (bool, error) {
			return false, claimErr
		},
	}
	svc := service.NewImportJobService(repo, nil)
	err := svc.ProcessImportJob(context.Background(), uuid.New())
	if !errors.Is(err, claimErr) {
		t.Fatalf("got %v", err)
	}
}

func TestProcessImportJob_notClaimed_completedJob(t *testing.T) {
	jid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	repo := &helpers.ImportJobRepository{
		ClaimFn: func(context.Context, uuid.UUID) (bool, error) {
			return false, nil
		},
		GetByIDFn: func(context.Context, uuid.UUID) (*domain.ImportJob, error) {
			return &domain.ImportJob{ID: jid, Status: domain.ImportStatusCompleted}, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	if err := svc.ProcessImportJob(context.Background(), jid); err != nil {
		t.Fatal(err)
	}
}

func TestProcessImportJob_notClaimed_getByIDFails(t *testing.T) {
	repo := &helpers.ImportJobRepository{
		ClaimFn: func(context.Context, uuid.UUID) (bool, error) {
			return false, nil
		},
		GetByIDFn: func(context.Context, uuid.UUID) (*domain.ImportJob, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc := service.NewImportJobService(repo, nil)
	if err := svc.ProcessImportJob(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func TestProcessImportJob_bucketUnsetMarksFailed(t *testing.T) {
	t.Setenv("IMPORT_S3_BUCKET", "")
	jid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	repo := &helpers.ImportJobRepository{
		ClaimFn: func(context.Context, uuid.UUID) (bool, error) {
			return true, nil
		},
		GetByIDFn: func(context.Context, uuid.UUID) (*domain.ImportJob, error) {
			return &domain.ImportJob{ID: jid, TenantID: "t", CsvS3Key: "csv"}, nil
		},
	}
	svc := service.NewImportJobService(repo, nil)
	if err := svc.ProcessImportJob(context.Background(), jid); err != nil {
		t.Fatal(err)
	}
	if len(repo.MarkFailedMsgs) != 1 || !strings.Contains(repo.MarkFailedMsgs[0], "IMPORT_S3_BUCKET") {
		t.Fatalf("markFailedMsgs=%v", repo.MarkFailedMsgs)
	}
}
