package main

import (
	"context"
	"encoding/json"
	"os"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"

	"project-serverless/internal/bootstrap"
	"project-serverless/internal/logger"
	"project-serverless/internal/service"
)

type queueBody struct {
	JobID string `json:"job_id"`
}

var importSvc service.ImportJobService

func setup() error {
	svc, err := bootstrap.SetupImportJobService()
	if err != nil {
		return err
	}
	importSvc = svc
	return nil
}

func processMessage(ctx context.Context, body string) error {
	var qb queueBody
	if err := json.Unmarshal([]byte(body), &qb); err != nil {
		logger.Error("import_worker_bad_message", map[string]any{"error": err.Error()})
		return nil
	}
	jobID, err := uuid.Parse(strings.TrimSpace(qb.JobID))
	if err != nil {
		return nil
	}
	return importSvc.ProcessImportJob(ctx, jobID)
}

func HandleRequest(ctx context.Context, ev events.SQSEvent) (events.SQSEventResponse, error) {
	var failures []events.SQSBatchItemFailure
	for _, rec := range ev.Records {
		if err := processMessage(ctx, rec.Body); err != nil {
			logger.Error("import_job_worker_record_failed", map[string]any{"error": err.Error(), "msg_id": rec.MessageId})
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: rec.MessageId})
		}
	}
	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}

func main() {
	logger.Info("booting_import_job_worker", map[string]any{"localstack_hostname": os.Getenv("LOCALSTACK_HOSTNAME")})
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
