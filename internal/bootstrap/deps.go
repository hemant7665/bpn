package bootstrap

import (
	"project-serverless/internal/db"
	"project-serverless/internal/repository"
	"project-serverless/internal/service"

	"github.com/joho/godotenv"
)

func SetupUserRepository() (repository.UserRepository, error) {
	_ = godotenv.Load()
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
