package main

import (
	"context"
	"runtime/debug"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/validator"
)

type CreateUserRequest struct {
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

func HandleRequest(ctx context.Context, req CreateUserRequest) (*domain.User, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, apperrors.NewValidation("name and valid email are required")
	}

	user := &domain.User{
		Name:  req.Name,
		Email: req.Email,
	}

	// 1. Write to DB
	if err := deps.repo.CreateUser(ctx, user); err != nil {
		return nil, apperrors.NewInternal("failed to create user", err)
	}

	logger.Info("user_created", map[string]any{"user_id": user.ID})

	// Publish domain and audit events to Kinesis
	if err := events.EmitUserEvents(ctx, "insert", *user); err != nil {
		return nil, apperrors.NewInternal("user created but event dispatch failed", err)
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
