package main

import (
	"context"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/service"
	"project-serverless/internal/validator"
)

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type dependencies struct {
	userService service.UserService
}

var deps dependencies

func setupDependencies() error {
	svc, err := bootstrap.SetupUserService()
	if err != nil {
		return svcerrors.Internal("database connection failed", err)
	}
	deps.userService = svc
	return nil
}

func HandleRequest(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	if err := validator.ValidateStruct(req); err != nil {
		return nil, svcerrors.Validation("valid email and password are required")
	}

	user, err := deps.userService.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return nil, svcerrors.NewUnauthorized(svcerrors.CodeInvalidCredential, "invalid credentials")
	}
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		return nil, svcerrors.NewUnauthorized(svcerrors.CodeInvalidCredential, "invalid credentials")
	}

	token, err := auth.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, svcerrors.Internal("failed to generate auth token", err)
	}

	return &LoginResponse{Token: token}, nil
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
