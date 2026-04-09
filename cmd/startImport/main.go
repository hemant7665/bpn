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
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

type request struct {
	Authorization string `json:"authorization"`
	JobID         string `json:"job_id"`
}

type response struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
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

func HandleRequest(ctx context.Context, req request) (*response, error) {
	tenantID, _, err := auth.ResolveTenant(ctx, req.Authorization, userRepo)
	if err != nil {
		return nil, err
	}
	jobID, err := uuid.Parse(strings.TrimSpace(req.JobID))
	if err != nil {
		return nil, svcerrors.ImportValidation("invalid job_id")
	}
	out, err := importSvc.StartImport(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}
	return &response{JobID: out.JobID, Status: out.Status}, nil
}

func main() {
	logger.Info("booting_start_import", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
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
