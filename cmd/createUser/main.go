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

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type dependencies struct {
	repo repository.UserRepository
}

var deps dependencies

func setupDependencies() error {
	_ = godotenv.Load()

	// DB Setup
	if _, err := db.Connect(); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	deps.repo = repository.NewUserRepository(db.GetDB())
	return nil
}

func HandleRequest(ctx context.Context, req CreateUserRequest) (*domain.User, error) {
	log.Printf("Received CreateUserRequest: Name=%s, Email=%s\n", req.Name, req.Email)

	if req.Name == "" || req.Email == "" {
		return nil, fmt.Errorf("name and email are required")
	}

	user := &domain.User{
		Name:  req.Name,
		Email: req.Email,
	}

	// 1. Write to DB
	if err := deps.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("User created in DB: ID=%d\n", user.ID)

	// Emit event to dedicated create queue → triggers userSyncWorker
	events.EmitEvent(ctx, events.CreateQueue, "insert", user.ID)

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
