package main

import (
	"context"
	"encoding/json"
	"os"
	"runtime/debug"

	lambdaevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
)

type Dependencies struct {
	UserRepository repository.UserRepository
}

var deps Dependencies

func HandleRequest(ctx context.Context, kinesisEvent lambdaevents.KinesisEvent) error {
	logger.Info("user_worker_received_event", map[string]any{"record_count": len(kinesisEvent.Records)})
	if len(kinesisEvent.Records) == 0 {
		return nil
	}

	for _, record := range kinesisEvent.Records {
		var payload events.UserEventPayload
		if err := json.Unmarshal(record.Kinesis.Data, &payload); err != nil {
			logger.Error("invalid_event_payload", map[string]any{"error": err.Error(), "event_id": record.EventID})
			return apperrors.NewInternal("failed to decode kinesis payload", err)
		}

		if payload.EventType != "domain" || payload.Entity != "user" {
			continue
		}

		switch payload.Operation {
		case "insert", "update":
			summary := &domain.UserSummary{
				ID:        payload.User.ID,
				Name:      payload.User.Name,
				Email:     payload.User.Email,
				CreatedAt: payload.User.CreatedAt,
			}
			if err := deps.UserRepository.UpsertUserSummary(ctx, summary); err != nil {
				logger.Error("projection_upsert_failed", map[string]any{"user_id": payload.User.ID, "error": err.Error()})
				return apperrors.NewInternal("failed to upsert read model projection", err)
			}
		case "delete":
			if err := deps.UserRepository.DeleteUserSummary(ctx, payload.User.ID); err != nil {
				logger.Error("projection_delete_failed", map[string]any{"user_id": payload.User.ID, "error": err.Error()})
				return apperrors.NewInternal("failed to delete read model projection", err)
			}
		default:
			logger.Info("ignored_unknown_operation", map[string]any{"operation": payload.Operation})
		}
	}

	logger.Info("read_model_projection_updated", nil)
	return nil
}

func setupDependencies() error {
	repo, err := bootstrap.SetupUserRepository()
	if err != nil {
		return apperrors.NewInternal("database connection failed", err)
	}
	deps.UserRepository = repo
	return nil
}

func main() {
	logger.Info("booting_user_sync_worker", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
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
