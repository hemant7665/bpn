package main

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	lambdaevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"

	"project-serverless/internal/db"
	"project-serverless/internal/repository"
)

type Dependencies struct {
	UserRepository repository.UserRepository
}

var (
	deps          Dependencies
	connectDBFunc = db.Connect
	getDBFunc     = db.GetDB
	lambdaStartFn = lambda.Start
)

func HandleRequest(ctx context.Context, sqsEvent lambdaevents.SQSEvent) (lambdaevents.SQSEventResponse, error) {
	log.Printf("User Worker received SQS event with %d records", len(sqsEvent.Records))

	// In a Materialized View pattern, any update to the base table requires a refresh.
	if len(sqsEvent.Records) > 0 {
		log.Println("Refreshing Materialized View: read_model.users_summary")
		if err := deps.UserRepository.RefreshView(ctx); err != nil {
			log.Printf("REFRESH_FAILURE: %v", err)
			// Return all records as failed so they retry
			var failures []lambdaevents.SQSBatchItemFailure
			for _, r := range sqsEvent.Records {
				failures = append(failures, lambdaevents.SQSBatchItemFailure{ItemIdentifier: r.MessageId})
			}
			return lambdaevents.SQSEventResponse{BatchItemFailures: failures}, nil
		}
		log.Println("Read model successfully updated.")
	}

	return lambdaevents.SQSEventResponse{}, nil
}

func setupDependencies() error {
	_ = godotenv.Load()
	if _, err := connectDBFunc(); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	dbInstance := getDBFunc()
	deps.UserRepository = repository.NewUserRepository(dbInstance)
	return nil
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("UNHANDLED PANIC: %v\nStack: %s", r, debug.Stack())
		}
	}()

	if err := setupDependencies(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	lambdaStartFn(HandleRequest)
}
