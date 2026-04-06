package main

import (
	"context"
	"os"
	"runtime/debug"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/service"
	"project-serverless/internal/validator"
)

type Dependencies struct {
	userService service.UserService
}

var deps Dependencies

type GetUserRequest struct {
	ID            interface{} `json:"id"`
	Authorization string      `json:"authorization"`
}

func setupDependencies() error {
	svc, err := bootstrap.SetupUserService()
	if err != nil {
		return svcerrors.Internal("database connection failed", err)
	}
	deps.userService = svc
	return nil
}

func HandleRequest(ctx context.Context, req GetUserRequest) (*domain.UserSummary, error) {
	if _, err := auth.AuthorizeHeader(req.Authorization); err != nil {
		return nil, svcerrors.Unauthorized("unauthorized")
	}
	idInt, err := validator.ParsePositiveIntID(req.ID)
	if err != nil {
		return nil, err
	}
	user, err := deps.userService.GetUser(ctx, idInt)
	if err != nil {
		return nil, svcerrors.NotFound("user not found")
	}
	return user, nil
}

func main() {
	logger.Info("booting_get_user_lambda", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
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
