package main

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"

	"project-serverless/internal/db"
	"project-serverless/internal/domain"
	"project-serverless/internal/events"
	"project-serverless/internal/repository"
)

type DeleteUserRequest struct {
	ID int `json:"id"`
}

type dependencies struct {
	repo repository.UserRepository
}

var deps dependencies

func setupDependencies() error {
	_ = godotenv.Load()

	if _, err := db.Connect(); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	deps.repo = repository.NewUserRepository(db.GetDB())
	return nil
}

func HandleRequest(ctx context.Context, req DeleteUserRequest) (*domain.User, error) {
	log.Printf("DeleteUser called: ID=%d", req.ID)

	if req.ID <= 0 {
		return nil, fmt.Errorf("id is required and must be > 0")
	}

	// Fetch from write model (source of truth — always consistent)
	var existing domain.User
	if err := db.GetDB().WithContext(ctx).Where("id = ?", req.ID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("user %d not found: %w", req.ID, err)
	}

	// Delete from write model
	if err := deps.repo.DeleteUser(ctx, req.ID); err != nil {
		return nil, fmt.Errorf("failed to delete user %d: %w", req.ID, err)
	}

	log.Printf("Successfully deleted user %d", req.ID)

	// Emit event to dedicated delete queue → triggers userSyncWorker
	events.EmitEvent(ctx, events.DeleteQueue, "delete", req.ID)

	return &existing, nil
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("UNHANDLED PANIC: %v\nStack: %s", r, debug.Stack())
		}
	}()

	if err := setupDependencies(); err != nil {
		log.Fatalf("Failed to initialize Lambda dependencies: %v", err)
	}

	lambda.Start(HandleRequest)
}
