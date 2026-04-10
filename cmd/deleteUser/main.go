package main

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
	"project-serverless/internal/validator"
)

type DeleteUserRequest struct {
	ID            int    `json:"id" validate:"required,gt=0"`
	Authorization string `json:"authorization" validate:"required"`
}

type auditPublisher interface {
	EmitUserAudit(ctx context.Context, operation string, user domain.User) error
}

type domainEventPublisher interface {
	PutRecordJSON(ctx context.Context, partitionKey string, payload []byte) error
}

type dependencies struct {
	userService     service.UserService
	userRepo        repository.UserRepository
	auditEmitter    auditPublisher
	domainPublisher domainEventPublisher
}

var deps dependencies

func setupDependencies() error {
	repo, err := bootstrap.SetupUserRepository()
	if err != nil {
		return svcerrors.Internal("database connection failed", err)
	}
	deps.userRepo = repo
	deps.userService = service.NewUserService(repo)
	auditEmitter, err := events.NewAuditEmitter(context.Background())
	if err != nil {
		return err
	}
	domainPub, err := events.NewDomainEventPublisher(context.Background())
	if err != nil {
		return err
	}
	deps.auditEmitter = auditEmitter
	deps.domainPublisher = domainPub
	return nil
}

func HandleRequest(ctx context.Context, req DeleteUserRequest) (*domain.User, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, svcerrors.Validation("id and authorization are required")
	}
	tenantID, _, err := auth.ResolveTenant(ctx, req.Authorization, deps.userRepo)
	if err != nil {
		return nil, err
	}

	existing, err := deps.userService.GetWriteUserByID(ctx, req.ID)
	if err != nil {
		return nil, svcerrors.NotFound("user not found")
	}
	if domain.NormalizeTenantID(existing.TenantID) != domain.NormalizeTenantID(tenantID) {
		return nil, svcerrors.NotFound("user not found")
	}

	if err := deps.userService.DeleteUser(ctx, req.ID); err != nil {
		return nil, svcerrors.Internal("failed to delete user", err)
	}
	logger.Info("user_deleted", map[string]any{"user_id": req.ID})

	if err := deps.auditEmitter.EmitUserAudit(ctx, "delete", *existing); err != nil {
		return nil, svcerrors.Internal("user deleted but audit emit failed", err)
	}

	ev := events.NewUserUpdatedEventFromUser(existing, true)
	payload, err := json.Marshal(ev)
	if err != nil {
		return nil, svcerrors.Internal("domain event marshal failed", err)
	}
	if err := deps.domainPublisher.PutRecordJSON(ctx, strconv.Itoa(existing.ID), payload); err != nil {
		logger.Info("domain_event_emit_failed", map[string]any{"error": err.Error(), "user_id": existing.ID})
	}

	return existing, nil
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("unhandled_panic", map[string]any{"panic": r, "stack": string(debug.Stack())})
		}
	}()

	if err := setupDependencies(); err != nil {
		logger.Error("failed_to_initialize_lambda_dependencies", map[string]any{"error": err.Error()})
		panic(err)
	}

	lambda.Start(HandleRequest)
}
