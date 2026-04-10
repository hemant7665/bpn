package main

import (
	"context"
	"testing"

	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
)

func TestProcessMessage_badJSON(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })
	importSvc = &helpers.ImportJobService{
		ProcessImportJobFn: func(context.Context, uuid.UUID) error {
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
	importSvc = &helpers.ImportJobService{
		ProcessImportJobFn: func(context.Context, uuid.UUID) error {
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
	importSvc = &helpers.ImportJobService{
		ProcessImportJobFn: func(_ context.Context, jobID uuid.UUID) error {
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
	importSvc = &helpers.ImportJobService{
		ProcessImportJobFn: func(context.Context, uuid.UUID) error {
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
	importSvc = &helpers.ImportJobService{
		ProcessImportJobFn: func(_ context.Context, jobID uuid.UUID) error {
			calls = append(calls, jobID)
			return nil
		},
	}

	ev := events.SQSEvent{Records: []events.SQSMessage{
		{MessageId: "m1", Body: `{"job_id":"` + j1.String() + `"}`},
		{MessageId: "m2", Body: `{"job_id":"` + j2.String() + `"}`},
	}}
	resp, err := HandleRequest(context.Background(), ev)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected no batch failures, got %+v", resp.BatchItemFailures)
	}
	if len(calls) != 2 || calls[0] != j1 || calls[1] != j2 {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestHandleRequest_batchItemFailureDoesNotBlockLaterRecords(t *testing.T) {
	old := importSvc
	t.Cleanup(func() { importSvc = old })

	j1 := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	j2 := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	var n int
	importSvc = &helpers.ImportJobService{
		ProcessImportJobFn: func(_ context.Context, jobID uuid.UUID) error {
			n++
			if jobID == j1 {
				return svcerrors.PlainMessage("fail first")
			}
			return nil
		},
	}

	ev := events.SQSEvent{Records: []events.SQSMessage{
		{MessageId: "fail-id", Body: `{"job_id":"` + j1.String() + `"}`},
		{MessageId: "ok-id", Body: `{"job_id":"` + j2.String() + `"}`},
	}}
	resp, err := HandleRequest(context.Background(), ev)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected both messages processed, n=%d", n)
	}
	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "fail-id" {
		t.Fatalf("expected single batch failure for fail-id, got %+v", resp.BatchItemFailures)
	}
}
