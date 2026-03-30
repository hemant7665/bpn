package services

import (
	"context"

	"project-serverless/internal/domain"
	"project-serverless/internal/repository"
)

type UserService interface {
	GetUser(ctx context.Context, id int) (*domain.UserSummary, error)
	ListUsers(ctx context.Context) ([]domain.UserSummary, error)
	CreateUser(ctx context.Context, name string, email string) (*domain.User, error)
	DeleteUser(ctx context.Context, id int) error
	UpdateUser(ctx context.Context, id int, name string, email string) (*domain.User, error)
}

type userServiceImpl struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userServiceImpl{repo: repo}
}

// GetUser reads from read_model.users_summary (materialized view).
func (s *userServiceImpl) GetUser(ctx context.Context, id int) (*domain.UserSummary, error) {
	return s.repo.GetUser(ctx, id)
}

// ListUsers reads all records from read_model.users_summary.
func (s *userServiceImpl) ListUsers(ctx context.Context) ([]domain.UserSummary, error) {
	return s.repo.ListUsers(ctx)
}

// CreateUser writes to write_model.users.
// ID is auto-assigned by the DB (SERIAL), so we don't set it here.
func (s *userServiceImpl) CreateUser(ctx context.Context, name string, email string) (*domain.User, error) {
	user := &domain.User{
		Name:  name,
		Email: email,
		// ID and CreatedAt are set by PostgreSQL
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// DeleteUser removes from write_model.users.
func (s *userServiceImpl) DeleteUser(ctx context.Context, id int) error {
	return s.repo.DeleteUser(ctx, id)
}

// UpdateUser patches write_model.users by fetching the current read summary first.
func (s *userServiceImpl) UpdateUser(ctx context.Context, id int, name string, email string) (*domain.User, error) {
	// Verify the user exists via the read model
	if _, err := s.repo.GetUser(ctx, id); err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:    id,
		Name:  name,
		Email: email,
	}

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
