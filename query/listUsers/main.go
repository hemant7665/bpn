package main

import (
	"context"
	"os"
	"runtime/debug"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/apperrors"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
)

type Dependencies struct {
	repo repository.UserRepository
}

var deps Dependencies

func setupDependencies() error {
	repo, err := bootstrap.SetupUserRepository()
	if err != nil {
		return apperrors.NewInternal("database connection failed", err)
	}
	deps.repo = repo
	return nil
}

func HandleRequest(ctx context.Context) ([]domain.UserSummary, error) {
	users, err := deps.repo.ListUsers(ctx)
	if err != nil {
		return nil, apperrors.NewInternal("list users failed", err)
	}
	return users, nil
}

func main() {
	logger.Info("booting_list_users_lambda", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
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
