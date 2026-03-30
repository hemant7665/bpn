package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/joho/godotenv"

	"project-serverless/internal/db"
	"project-serverless/internal/repository"
)

type Dependencies struct {
	repo repository.UserRepository
}

var deps Dependencies

func setupDependencies() error {
	if _, err := db.Connect(); err != nil {
		return fmt.Errorf("DATABASE_CONNECT_ERROR: %w", err)
	}
	deps.repo = repository.NewUserRepository(db.GetDB())
	return nil
}

type GetUserRequest struct {
	ID interface{} `json:"id"` // accepts both string ("7") and int (7)
}

func parseID(raw interface{}) (int, error) {
	switch v := raw.(type) {
	case float64: // JSON numbers unmarshal as float64
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unsupported id type: %T", raw)
	}
}

func HandleRequest(ctx context.Context, req GetUserRequest) (interface{}, error) {
	log.Printf("GetUser called with ID: %v", req.ID)

	if req.ID == nil {
		return nil, fmt.Errorf("ID is required")
	}

	idInt, err := parseID(req.ID)
	if err != nil {
		return nil, fmt.Errorf("INVALID_ID_FORMAT: %v (must be a number)", req.ID)
	}

	user, err := deps.repo.GetUser(ctx, idInt)
	if err != nil {
		log.Printf("DB_QUERY_FAILURE: %v", err)
		return nil, fmt.Errorf("USER_NOT_FOUND: %w", err)
	}

	return user, nil
}

func main() {
	log.Printf("BOOTING getUser Lambda (LOCALSTACK_HOSTNAME=%s)", os.Getenv("LOCALSTACK_HOSTNAME"))

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "EARLY_BOOT_PANIC: %v\nStack: %s", r, debug.Stack())
		}
	}()

	// Load .env — safe to call even if file is absent (Lambda env vars take over)
	_ = godotenv.Load()

	if err := setupDependencies(); err != nil {
		log.Fatalf("Failed to initialize Lambda dependencies: %v", err)
	}

	lambda.Start(HandleRequest)
}
