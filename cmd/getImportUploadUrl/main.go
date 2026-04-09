package main

import (
	"context"
	"os"
	"runtime/debug"

	"github.com/aws/aws-lambda-go/lambda"

	"project-serverless/internal/auth"
	"project-serverless/internal/bootstrap"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

type request struct {
	Authorization string `json:"authorization"`
}

type response struct {
	URL              string `json:"url"`
	JobID            string `json:"job_id"`
	CsvS3Key         string `json:"csv_s3_key"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
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
	tenantID, userID, err := auth.ResolveTenant(ctx, req.Authorization, userRepo)
	if err != nil {
		return nil, err
	}
	out, err := importSvc.CreatePendingJobWithPresignedPut(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return &response{
		URL:              out.URL,
		JobID:            out.JobID,
		CsvS3Key:         out.CsvS3Key,
		ExpiresInSeconds: out.ExpiresInSeconds,
	}, nil
}

func main() {
	logger.Info("booting_get_import_upload_url", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
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
