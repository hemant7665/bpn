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

type DeleteUserRequest struct {
	ID int `json:"id" validate:"required,gt=0"`
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

func HandleRequest(ctx context.Context, req DeleteUserRequest) (*domain.User, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, apperrors.NewValidation("id must be greater than 0")
	}

	// Fetch from write model (source of truth — always consistent)
	var existing domain.User
	if err := db.GetDB().WithContext(ctx).Where("id = ?", req.ID).First(&existing).Error; err != nil {
		return nil, apperrors.NewNotFound("user not found")
	}

	// Delete from write model
	if err := deps.repo.DeleteUser(ctx, req.ID); err != nil {
		return nil, apperrors.NewInternal("failed to delete user", err)
	}
	logger.Info("user_deleted", map[string]any{"user_id": req.ID})

	// Publish domain and audit events to Kinesis
	if err := events.EmitUserEvents(ctx, "delete", existing); err != nil {
		return nil, apperrors.NewInternal("user deleted but event dispatch failed", err)
	}

	return &existing, nil
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
