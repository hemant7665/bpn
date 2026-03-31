package main

import (
	"context"
	"runtime/debug"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/db"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/validator"
)

type UpdateUserRequest struct {
	ID    int    `json:"id" validate:"required,gt=0"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type dependencies struct {
	repo repository.UserRepository
}

var deps dependencies

func setupDependencies() error {
	repo, err := bootstrap.SetupUserRepository()
	if err != nil {
		return apperrors.NewInternal("database connection failed", err)
	}
	deps.repo = repo
	return nil
}

func HandleRequest(ctx context.Context, req UpdateUserRequest) (*domain.User, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, apperrors.NewValidation("id>0, name, and valid email are required")
	}

	// Verify user exists in write model (source of truth)
	var existing domain.User
	if err := db.GetDB().WithContext(ctx).Where("id = ?", req.ID).First(&existing).Error; err != nil {
		return nil, apperrors.NewNotFound("user not found")
	}

	user := &domain.User{
		ID:    req.ID,
		Name:  req.Name,
		Email: req.Email,
		CreatedAt: existing.CreatedAt,
	}

	if err := deps.repo.UpdateUser(ctx, user); err != nil {
		return nil, apperrors.NewInternal("failed to update user", err)
	}
	logger.Info("user_updated", map[string]any{"user_id": req.ID})

	// Publish domain and audit events to Kinesis
	if err := events.EmitUserEvents(ctx, "update", *user); err != nil {
		return nil, apperrors.NewInternal("user updated but event dispatch failed", err)
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
