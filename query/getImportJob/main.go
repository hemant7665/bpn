package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

type request struct {
	Authorization string `json:"authorization"`
	JobID         string `json:"job_id"`
}

var (
	importSvc service.ImportJobService
	userRepo  repository.UserRepository
)

func setup() error {
	svc, err := bootstrap.SetupImportJobService()
	if err != nil {
		return err
	}
	ur, err := bootstrap.SetupUserRepository()
	if err != nil {
		return err
	}
	importSvc = svc
	userRepo = ur
	return nil
}

func HandleRequest(ctx context.Context, req request) (*domain.ImportJob, error) {
	tenantID, _, err := auth.ResolveTenant(ctx, req.Authorization, userRepo)
	if err != nil {
		return nil, err
	}
	jobID, err := uuid.Parse(strings.TrimSpace(req.JobID))
	if err != nil {
		return nil, svcerrors.ImportValidation("invalid job_id")
	}
	return importSvc.GetJobForTenant(ctx, tenantID, jobID)
}

func main() {
	logger.Info("booting_get_import_job", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
	defer func() {
		if r := recover(); r != nil {
			logger.Error("unhandled_panic", map[string]any{"panic": r, "stack": string(debug.Stack())})
		}
	}()
	if err := setup(); err != nil {
		logger.Error("failed_to_initialize_lambda_dependencies", map[string]any{"error": err.Error()})
		panic(err)
	}
	lambda.Start(HandleRequest)
}
