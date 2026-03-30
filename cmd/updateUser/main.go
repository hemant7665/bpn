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

type UpdateUserRequest struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
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

func HandleRequest(ctx context.Context, req UpdateUserRequest) (*domain.User, error) {
	log.Printf("UpdateUser called: ID=%d Name=%s Email=%s", req.ID, req.Name, req.Email)

	if req.ID <= 0 {
		return nil, fmt.Errorf("id is required and must be > 0")
	}
	if req.Name == "" || req.Email == "" {
		return nil, fmt.Errorf("name and email are required")
	}

	// Verify user exists in write model (source of truth)
	var existing domain.User
	if err := db.GetDB().WithContext(ctx).Where("id = ?", req.ID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("user %d not found: %w", req.ID, err)
	}

	user := &domain.User{
		ID:    req.ID,
		Name:  req.Name,
		Email: req.Email,
	}

	if err := deps.repo.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user %d: %w", req.ID, err)
	}

	log.Printf("Successfully updated user %d", req.ID)

	// Emit event to dedicated update queue → triggers userSyncWorker
	events.EmitEvent(ctx, events.UpdateQueue, "update", req.ID)

	return user, nil
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
