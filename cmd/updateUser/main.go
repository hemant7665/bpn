package main

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/service"
	"project-serverless/internal/validator"
)

type UpdateUserRequest struct {
	ID            int    `json:"id" validate:"required,gt=0"`
	Username      string `json:"username" validate:"required"`
	Email         string `json:"email" validate:"required,email"`
	PhoneNo       string `json:"phone_no"`
	DateOfBirth   string `json:"date_of_birth"`
	Gender        string `json:"gender"`
	Authorization string `json:"authorization" validate:"required"`
}

type dependencies struct {
	userService       service.UserService
	auditEmitter      auditPublisher
	domainPublisher   domainEventPublisher
}

type auditPublisher interface {
	EmitUserAudit(ctx context.Context, operation string, user domain.User) error
}

type domainEventPublisher interface {
	PutRecordJSON(ctx context.Context, partitionKey string, payload []byte) error
}

var deps dependencies

func setupDependencies() error {
	svc, err := bootstrap.SetupUserService()
	if err != nil {
		return svcerrors.Internal("database connection failed", err)
	}
	auditEmitter, err := events.NewAuditEmitter(context.Background())
	if err != nil {
		return err
	}
	domainPub, err := events.NewDomainEventPublisher(context.Background())
	if err != nil {
		return err
	}
	deps.userService = svc
	deps.auditEmitter = auditEmitter
	deps.domainPublisher = domainPub
	return nil
}

func HandleRequest(ctx context.Context, req UpdateUserRequest) (*domain.User, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, svcerrors.Validation("id, username, valid email and authorization are required")
	}
	if _, err := auth.AuthorizeHeader(req.Authorization); err != nil {
		return nil, svcerrors.Unauthorized("unauthorized")
	}

	existing, err := deps.userService.GetWriteUserByID(ctx, req.ID)
	if err != nil {
		return nil, svcerrors.NotFound("user not found")
	}

	var dob *time.Time
	if strings.TrimSpace(req.DateOfBirth) != "" {
		t, perr := time.Parse("2006-01-02", strings.TrimSpace(req.DateOfBirth))
		if perr != nil {
			return nil, svcerrors.Validation("date_of_birth must be YYYY-MM-DD")
		}
		dob = &t
	} else {
		dob = existing.DateOfBirth
	}

	user := &domain.User{
		ID:           req.ID,
		TenantID:     existing.TenantID,
		Username:     strings.TrimSpace(req.Username),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PhoneNo:      strings.TrimSpace(req.PhoneNo),
		DateOfBirth:  dob,
		Gender:       strings.TrimSpace(req.Gender),
		PasswordHash: existing.PasswordHash,
		CreatedAt:    existing.CreatedAt,
	}

	if err := deps.userService.UpdateUser(ctx, user); err != nil {
		return nil, svcerrors.Internal("failed to update user", err)
	}
	logger.Info("user_updated", map[string]any{"user_id": req.ID})

	if err := deps.auditEmitter.EmitUserAudit(ctx, "update", *user); err != nil {
		return nil, svcerrors.Internal("user updated but audit emit failed", err)
	}

	ev := events.NewUserUpdatedEventFromUser(user, false)
	payload, err := json.Marshal(ev)
	if err != nil {
		return nil, svcerrors.Internal("domain event marshal failed", err)
	}
	if err := deps.domainPublisher.PutRecordJSON(ctx, strconv.Itoa(user.ID), payload); err != nil {
		logger.Info("domain_event_emit_failed", map[string]any{"error": err.Error(), "user_id": user.ID})
	}

	return user, nil
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
