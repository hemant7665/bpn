package main

import (
	"context"
	"encoding/json"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	lambdaevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"

	"project-serverless/internal/bootstrap"
	"project-serverless/internal/cdcutil"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	appevents "project-serverless/internal/events"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
)

type dependencies struct {
	UserRepository  repository.UserRepository
	DomainPublisher *appevents.DomainEventPublisher
}

var deps dependencies

type cdcSQSMessage struct {
	Metadata struct {
		Operation string `json:"operation"`
		Table     string `json:"table-name"`
		Schema    string `json:"schema-name"`
	} `json:"metadata"`
	Data map[string]any `json:"data"`
}

func HandleRequest(ctx context.Context, sqsEvent lambdaevents.SQSEvent) (lambdaevents.SQSEventResponse, error) {
	var failures []lambdaevents.SQSBatchItemFailure
	for _, rec := range sqsEvent.Records {
		if err := processSQSEvent(ctx, rec.Body); err != nil {
			logger.Error("user_event_worker_record_failed", map[string]any{"error": err.Error(), "msg_id": rec.MessageId})
			failures = append(failures, lambdaevents.SQSBatchItemFailure{ItemIdentifier: rec.MessageId})
		}
	}
	return lambdaevents.SQSEventResponse{BatchItemFailures: failures}, nil
}

func processSQSEvent(ctx context.Context, body string) error {
	var typeProbe struct {
		EventType string `json:"eventType"`
	}
	if err := json.Unmarshal([]byte(body), &typeProbe); err != nil {
		return svcerrors.Internal("invalid SQS body", err)
	}

	switch strings.TrimSpace(typeProbe.EventType) {
	case appevents.UserReadModelSyncEventType:
		if err := deps.UserRepository.RefreshUsersSummaryView(ctx); err != nil {
			return svcerrors.Internal("failed to refresh read model materialized view", err)
		}
		logger.Info("user_event_worker_refreshed_mv", map[string]any{"source": appevents.UserReadModelSyncEventType})
		return nil

	case appevents.UserCreatedEventType:
		var ev appevents.UserCreatedEvent
		if err := json.Unmarshal([]byte(body), &ev); err != nil {
			return svcerrors.Internal("invalid UserCreated payload", err)
		}
		return handleUserCreated(ctx, &ev)

	case appevents.UserUpdatedEventType:
		var ev appevents.UserUpdatedEvent
		if err := json.Unmarshal([]byte(body), &ev); err != nil {
			return svcerrors.Internal("invalid UserUpdated payload", err)
		}
		return handleUserUpdated(ctx, &ev)
	}

	var cdc cdcSQSMessage
	if err := json.Unmarshal([]byte(body), &cdc); err != nil {
		return svcerrors.Internal("invalid SQS body", err)
	}
	if cdc.Data == nil || !cdcTableIsUsers(cdc.Metadata.Table) {
		logger.Info("user_event_worker_skip_unknown", map[string]any{"hint": "not_users_cdc"})
		return nil
	}
	if !schemaAllowsWriteModelUsers(cdc.Metadata.Schema) {
		logger.Info("user_event_worker_skip_schema", map[string]any{"schema": cdc.Metadata.Schema})
		return nil
	}

	return handleCDCEvent(ctx, cdc.Data)
}

// handleCDCEvent: refresh MV first, then best-effort domain Kinesis (publish failures do not fail SQS after refresh).
func handleCDCEvent(ctx context.Context, data map[string]any) error {
	if err := deps.UserRepository.RefreshUsersSummaryView(ctx); err != nil {
		return svcerrors.Internal("failed to refresh read model materialized view", err)
	}

	userID, err := cdcutil.StringFromData(data, "id")
	if err != nil {
		logger.Info("user_event_worker_cdc_skip_domain_publish", map[string]any{"reason": "id", "error": err.Error()})
		return nil
	}
	tenantID, err := cdcutil.StringFromData(data, "tenant_id")
	if err != nil {
		logger.Info("user_event_worker_cdc_skip_domain_publish", map[string]any{"reason": "tenant_id", "error": err.Error()})
		return nil
	}
	email, err := cdcutil.StringFromData(data, "email")
	if err != nil {
		logger.Info("user_event_worker_cdc_skip_domain_publish", map[string]any{"reason": "email", "error": err.Error()})
		return nil
	}
	username, err := cdcutil.StringFromData(data, "username")
	if err != nil {
		logger.Info("user_event_worker_cdc_skip_domain_publish", map[string]any{"reason": "username", "error": err.Error()})
		return nil
	}
	role, _ := cdcutil.StringFromData(data, "role")

	if deps.DomainPublisher == nil {
		logger.Info("user_event_worker_cdc_refreshed_no_domain_stream", map[string]any{"user_id": userID})
		return nil
	}

	now := time.Now().UTC()
	ev := appevents.UserCreatedEvent{
		EventType:  appevents.UserCreatedEventType,
		EventID:    "evt_" + uuid.New().String(),
		Version:    appevents.UserEventVersion,
		UserID:     userID,
		TenantID:   tenantID,
		Email:      email,
		Username:   username,
		Role:       role,
		CreatedAt:  now,
		OccurredAt: now,
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		logger.Info("user_event_worker_domain_publish_failed", map[string]any{"error": err.Error()})
		return nil
	}
	if err := deps.DomainPublisher.PutRecordJSON(ctx, userID, payload); err != nil {
		logger.Info("user_event_worker_domain_publish_failed", map[string]any{"error": err.Error(), "user_id": userID})
		return nil
	}
	logger.Info("user_event_worker_cdc_published", map[string]any{"user_id": userID})
	return nil
}

func handleUserCreated(ctx context.Context, ev *appevents.UserCreatedEvent) error {
	uid, _ := strconv.Atoi(ev.UserID)
	u := &domain.User{
		ID:        uid,
		TenantID:  ev.TenantID,
		Email:     ev.Email,
		Username:  ev.Username,
		CreatedAt: ev.CreatedAt,
	}
	if err := deps.UserRepository.SaveUserReadModel(ctx, u); err != nil {
		return svcerrors.Internal("failed to save UserCreated to read model", err)
	}
	logger.Info("user_event_worker_domain_user_created", map[string]any{"user_id": ev.UserID})
	return nil
}

func handleUserUpdated(ctx context.Context, ev *appevents.UserUpdatedEvent) error {
	uid, _ := strconv.Atoi(ev.UserID)
	u := &domain.User{
		ID:        uid,
		TenantID:  ev.TenantID,
		Email:     ev.Email,
		Username:  ev.Username,
		CreatedAt: ev.UpdatedAt,
	}
	if err := deps.UserRepository.SaveUserReadModel(ctx, u); err != nil {
		return svcerrors.Internal("failed to save UserUpdated to read model", err)
	}
	logger.Info("user_event_worker_domain_user_updated", map[string]any{"user_id": ev.UserID})
	return nil
}

func schemaAllowsWriteModelUsers(schema string) bool {
	s := strings.TrimSpace(schema)
	if s == "" || strings.EqualFold(s, "write_model") {
		return true
	}
	if os.Getenv("ENVIRONMENT") == "local" && strings.EqualFold(s, "public") {
		return true
	}
	return false
}

func cdcTableIsUsers(table string) bool {
	return strings.EqualFold(strings.TrimSpace(table), "users")
}

func setupDependencies() error {
	repo, err := bootstrap.SetupUserRepository()
	if err != nil {
		return svcerrors.Internal("database connection failed", err)
	}
	deps.UserRepository = repo
	pub, err := appevents.NewDomainEventPublisher(context.Background())
	if err != nil {
		logger.Info("user_event_worker_domain_publisher_disabled", map[string]any{"error": err.Error()})
		deps.DomainPublisher = nil
		return nil
	}
	deps.DomainPublisher = pub
	return nil
}

func main() {
	logger.Info("booting_user_event_worker", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
	defer func() {
		if r := recover(); r != nil {
			logger.Error("unhandled_panic", map[string]any{"panic": r, "stack": string(debug.Stack())})
		}
	}()
	if err := setupDependencies(); err != nil {
		logger.Error("failed_to_initialize_worker", map[string]any{"error": err.Error()})
		panic(err)
	}
	lambda.Start(HandleRequest)
}
