package main

import (
	"context"
	"encoding/json"
	"errors"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"gorm.io/gorm"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/service"
	"project-serverless/internal/validator"
)

type CreateUserRequest struct {
	TenantID     string `json:"tenant_id"`
	Username     string `json:"username" validate:"required"`
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
	PhoneNo      string `json:"phone_no"`
	DateOfBirth  string `json:"date_of_birth"`
	Gender       string `json:"gender"`
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

func HandleRequest(ctx context.Context, req CreateUserRequest) (*domain.User, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, svcerrors.Validation("username, valid email and password (min 8) are required")
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, svcerrors.Internal("failed to hash password", err)
	}

	tid := strings.TrimSpace(req.TenantID)
	if tid == "" {
		tid = "default-tenant"
	}

	var dob *time.Time
	if strings.TrimSpace(req.DateOfBirth) != "" {
		t, perr := time.Parse("2006-01-02", strings.TrimSpace(req.DateOfBirth))
		if perr != nil {
			return nil, svcerrors.Validation("date_of_birth must be YYYY-MM-DD")
		}
		dob = &t
	}

	user := &domain.User{
		TenantID:     tid,
		Username:     strings.TrimSpace(req.Username),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PhoneNo:      strings.TrimSpace(req.PhoneNo),
		DateOfBirth:  dob,
		Gender:       strings.TrimSpace(req.Gender),
		PasswordHash: passwordHash,
	}

	if err := deps.userService.CreateUser(ctx, user); err != nil {
		if errors.Is(err, svcerrors.ErrEmailAlreadyExists) {
			return nil, svcerrors.Conflict("user with this email already exists for this tenant")
		}
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, svcerrors.Conflict("user with this email already exists for this tenant")
		}
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "duplicate key") ||
			strings.Contains(low, "unique constraint") ||
			strings.Contains(low, "violates unique constraint") ||
			strings.Contains(low, "23505") {
			return nil, svcerrors.Conflict("user with this email already exists for this tenant")
		}
		logger.Error("create_user_db_failed", map[string]any{"error": err.Error()})
		// Include DB text in message so Lambda errorMessage / GraphQL shows the real failure (redeploy required).
		return nil, svcerrors.Internal("failed to create user: "+err.Error(), err)
	}

	logger.Info("user_created", map[string]any{"user_id": user.ID})

	if err := deps.auditEmitter.EmitUserAudit(ctx, "insert", *user); err != nil {
		return nil, svcerrors.Internal("user created but audit emit failed", err)
	}

	ev := events.NewUserCreatedEventFromUser(user)
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
