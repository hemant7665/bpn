package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	lambdaevents "github.com/aws/aws-lambda-go/events"

	"project-serverless/internal/domain"
	appevents "project-serverless/internal/events"
	"project-serverless/internal/helpers"
)

func TestSchemaAllowsWriteModelUsers_emptyAndWriteModel(t *testing.T) {
	t.Setenv("ENVIRONMENT", "")
	if !schemaAllowsWriteModelUsers("") || !schemaAllowsWriteModelUsers("write_model") {
		t.Fatal()
	}
}

func TestSchemaAllowsWriteModelUsers_publicWhenLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")
	if !schemaAllowsWriteModelUsers("public") {
		t.Fatal("expected public allowed in local")
	}
}

func TestSchemaAllowsWriteModelUsers_rejectPublicWhenNotLocal(t *testing.T) {
	t.Setenv("ENVIRONMENT", "staging")
	if schemaAllowsWriteModelUsers("public") {
		t.Fatal("public must not be allowed outside local")
	}
}

func TestCdcTableIsUsers(t *testing.T) {
	if !cdcTableIsUsers("users") || !cdcTableIsUsers(" USERS ") {
		t.Fatal()
	}
	if cdcTableIsUsers("other") {
		t.Fatal()
	}
}

func TestProcessSQSEvent_invalidJSON(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	err := processSQSEvent(context.Background(), `{`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessSQSEvent_userReadModelSync(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	body := `{"eventType":"` + appevents.UserReadModelSyncEventType + `"}`
	if err := processSQSEvent(context.Background(), body); err != nil {
		t.Fatal(err)
	}
}

func TestProcessSQSEvent_userReadModelSync_refreshError(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{
		UserRepository: &helpers.UserRepository{
			RefreshUsersSummaryViewFn: func(context.Context) error { return errTest },
		},
		DomainPublisher: nil,
	}

	body := `{"eventType":"` + appevents.UserReadModelSyncEventType + `"}`
	if err := processSQSEvent(context.Background(), body); err == nil {
		t.Fatal("expected error")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestProcessSQSEvent_userCreated(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	ev := appevents.UserCreatedEvent{
		EventType:  appevents.UserCreatedEventType,
		EventID:    "evt_x",
		Version:    appevents.UserEventVersion,
		UserID:     "42",
		TenantID:   "t1",
		Email:      "a@b.c",
		Username:   "u",
		CreatedAt:  time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
		OccurredAt: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := processSQSEvent(context.Background(), string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestProcessSQSEvent_userCreated_saveError(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{
		SaveUserReadModelFn: func(context.Context, *domain.User) error { return errTest },
	}, DomainPublisher: nil}

	ev := appevents.UserCreatedEvent{
		EventType: appevents.UserCreatedEventType,
		UserID:    "1",
		TenantID:  "t",
		Email:     "a@b.c",
		Username:  "u",
		CreatedAt: time.Now().UTC(),
	}
	raw, _ := json.Marshal(ev)
	if err := processSQSEvent(context.Background(), string(raw)); err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessSQSEvent_userUpdated(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	ev := appevents.UserUpdatedEvent{
		EventType: appevents.UserUpdatedEventType,
		UserID:    "7",
		TenantID:  "t1",
		Email:     "a@b.c",
		Username:  "u",
		UpdatedAt: time.Date(2021, 2, 3, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := processSQSEvent(context.Background(), string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestProcessSQSEvent_cdcSkipUnknownTable(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	body := `{"metadata":{"table-name":"orders","schema-name":"write_model"},"data":{"id":"1"}}`
	if err := processSQSEvent(context.Background(), body); err != nil {
		t.Fatal(err)
	}
}

func TestProcessSQSEvent_cdcSkipWrongSchema(t *testing.T) {
	t.Setenv("ENVIRONMENT", "staging")
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	body := `{"metadata":{"table-name":"users","schema-name":"analytics"},"data":{"id":"1"}}`
	if err := processSQSEvent(context.Background(), body); err != nil {
		t.Fatal(err)
	}
}

func TestProcessSQSEvent_cdcUsersWriteModel_refreshOnly(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	body := `{
		"metadata": {"table-name": "users", "schema-name": "write_model", "operation": "insert"},
		"data": {"id": "99", "tenant_id": "t", "email": "e@e.com", "username": "uname"}
	}`
	if err := processSQSEvent(context.Background(), body); err != nil {
		t.Fatal(err)
	}
}

func TestHandleRequest_batchItemFailure(t *testing.T) {
	old := deps
	defer func() { deps = old }()
	deps = dependencies{UserRepository: &helpers.UserRepository{}, DomainPublisher: nil}

	ev := lambdaevents.SQSEvent{Records: []lambdaevents.SQSMessage{
		{MessageId: "m-bad", Body: `{`},
		{MessageId: "m-ok", Body: `{"eventType":"` + appevents.UserReadModelSyncEventType + `"}`},
	}}
	resp, err := HandleRequest(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "m-bad" {
		t.Fatalf("failures: %+v", resp.BatchItemFailures)
	}
}
