package bootstrap

import (
	"context"
	"errors"
	"os"

	"github.com/joho/godotenv"

	"project-serverless/internal/db"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/logger"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"
)

// loadDotenvOptional loads ".env" when present (local dev). Missing file is OK (e.g. Lambda uses real env vars).
// Any other read/parse error is returned with a stable app error code.
func loadDotenvOptional() error {
	err := godotenv.Load()
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var pe *os.PathError
	if errors.As(err, &pe) && errors.Is(pe.Err, os.ErrNotExist) {
		return nil
	}
	logger.Error("bootstrap_dotenv_failed", map[string]any{"error": err.Error()})
	return svcerrors.Internal("failed to load .env file", err)
}

func SetupUserRepository() (repository.UserRepository, error) {
	if err := loadDotenvOptional(); err != nil {
		return nil, err
	}
	if _, err := db.Connect(); err != nil {
		return nil, err
	}
	return repository.NewUserRepository(db.GetDB()), nil
}

// SetupUserService returns the user application service for Lambda handlers (command/query).
func SetupUserService() (service.UserService, error) {
	repo, err := SetupUserRepository()
	if err != nil {
		return nil, err
	}
	return service.NewUserService(repo), nil
}

// SetupImportJobRepository returns the import_jobs repository (requires DB connected).
func SetupImportJobRepository() (repository.ImportJobRepository, error) {
	if err := loadDotenvOptional(); err != nil {
		return nil, err
	}
	if _, err := db.Connect(); err != nil {
		return nil, err
	}
	return repository.NewImportJobRepository(db.GetDB()), nil
}

// SetupImportJobService returns the import application service for Lambda handlers (same pattern as SetupUserService).
func SetupImportJobService() (service.ImportJobService, error) {
	repo, err := SetupImportJobRepository()
	if err != nil {
		return nil, err
	}
	awsClients, err := service.NewImportJobAWSFromDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return service.NewImportJobService(repo, awsClients), nil
}
