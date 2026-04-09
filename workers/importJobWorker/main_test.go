package main

import (
	"context"
	"testing"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/service"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
)

type workerImportSvcMock struct {
	processFn func(ctx context.Context, jobID uuid.UUID) error
}

func (m workerImportSvcMock) CreatePendingJobWithPresignedPut(context.Context, string, int) (*service.ImportUploadURLResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m workerImportSvcMock) StartImport(context.Context, string, uuid.UUID) (*service.ImportStartResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m workerImportSvcMock) GetJobForTenant(context.Context, string, uuid.UUID) (*domain.ImportJob, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m workerImportSvcMock) ListJobsForTenant(context.Context, string, int, int, *string, string) ([]domain.ImportJob, int64, error) {
	return nil, 0, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m workerImportSvcMock) PresignReportGetURL(context.Context, string, uuid.UUID) (*service.ImportReportPresignResult, error) {
	return nil, svcerrors.NewInternal(svcerrors.CodeInternal, "not implemented", nil)
}
func (m workerImportSvcMock) ProcessImportJob(ctx context.Context, jobID uuid.UUID) error {
	if m.processFn != nil {
		return m.processFn(ctx, jobID)
	}
	return nil
}

func TestProcessMessage_badJSON(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })
	importSvc = workerImportSvcMock{
		processFn: func(context.Context, uuid.UUID) error {
			t.Fatal("ProcessImportJob should not be called for bad JSON")
			return nil
		},
	}
	if err := processMessage(context.Background(), "not json"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestProcessMessage_invalidUUID(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })
	importSvc = workerImportSvcMock{
		processFn: func(context.Context, uuid.UUID) error {
			t.Fatal("ProcessImportJob should not be called for invalid uuid")
			return nil
		},
	}
	if err := processMessage(context.Background(), `{"job_id":"nope"}`); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestProcessMessage_success(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })

	jid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	var saw uuid.UUID
	importSvc = workerImportSvcMock{
		processFn: func(_ context.Context, jobID uuid.UUID) error {
			saw = jobID
			return nil
		},
	}
	if err := processMessage(context.Background(), `{"job_id":"`+jid.String()+`"}`); err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if saw != jid {
		t.Fatalf("expected job %v, got %v", jid, saw)
	}
}

func TestProcessMessage_processErrorPropagates(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })

	jid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	importSvc = workerImportSvcMock{
		processFn: func(context.Context, uuid.UUID) error {
			return svcerrors.PlainMessage("db down")
		},
	}
	err := processMessage(context.Background(), `{"job_id":"`+jid.String()+`"}`)
	if err == nil || err.Error() != "db down" {
		t.Fatalf("expected db down error, got %v", err)
	}
}

func TestHandleRequest_multipleRecords(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })

	j1 := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	j2 := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	var calls []uuid.UUID
	importSvc = workerImportSvcMock{
		processFn: func(_ context.Context, jobID uuid.UUID) error {
			calls = append(calls, jobID)
			return nil
		},
	}

	ev := events.SQSEvent{Records: []events.SQSMessage{
		{Body: `{"job_id":"` + j1.String() + `"}`},
		{Body: `{"job_id":"` + j2.String() + `"}`},
	}}
	if err := HandleRequest(context.Background(), ev); err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if len(calls) != 2 || calls[0] != j1 || calls[1] != j2 {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestHandleRequest_stopsOnFirstProcessError(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })

	j1 := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	j2 := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	var n int
	importSvc = workerImportSvcMock{
		processFn: func(_ context.Context, jobID uuid.UUID) error {
			n++
			if jobID == j1 {
				return svcerrors.PlainMessage("fail first")
			}
			return nil
		},
	}

	ev := events.SQSEvent{Records: []events.SQSMessage{
		{Body: `{"job_id":"` + j1.String() + `"}`},
		{Body: `{"job_id":"` + j2.String() + `"}`},
	}}
	err := HandleRequest(context.Background(), ev)
	if err == nil || err.Error() != "fail first" {
		t.Fatalf("expected fail first, got %v", err)
	}
	if n != 1 {
		t.Fatalf("expected only first message processed, n=%d", n)
	}
}
