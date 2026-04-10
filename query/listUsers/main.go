package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

type Dependencies struct {
	userService service.UserService
	userRepo    repository.UserRepository
}

var deps Dependencies

const (
	defaultLimit = 10
	maxLimit     = 100
)

type ListUsersRequest struct {
	Skip          int    `json:"skip"`
	Limit         int    `json:"limit"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	Authorization string `json:"authorization"`
}

type ListUsersResponse struct {
	Items []domain.UserSummary `json:"items"`
	Total int64                `json:"total"`
}

func setupDependencies() error {
	repo, err := bootstrap.SetupUserRepository()
	if err != nil {
		return svcerrors.Internal("database connection failed", err)
	}
	deps.userRepo = repo
	deps.userService = service.NewUserService(repo)
	return nil
}

func HandleRequest(ctx context.Context, req ListUsersRequest) (*ListUsersResponse, error) {
	tenantID, _, err := auth.ResolveTenant(ctx, req.Authorization, deps.userRepo)
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 {
		return nil, svcerrors.Validation("limit must be greater than 0")
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if req.Skip < 0 {
		return nil, svcerrors.Validation("skip must be >= 0")
	}

	filter := domain.ListUsersFilter{}
	if strings.TrimSpace(req.Username) != "" {
		u := strings.TrimSpace(req.Username)
		filter.Username = &u
	}
	if strings.TrimSpace(req.Email) != "" {
		e := strings.TrimSpace(req.Email)
		filter.Email = &e
	}

	items, total, err := deps.userService.ListUsersFiltered(ctx, tenantID, req.Skip, limit, filter)
	if err != nil {
		return nil, svcerrors.Internal("list users failed", err)
	}
	return &ListUsersResponse{Items: items, Total: total}, nil
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
