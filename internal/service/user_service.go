package service

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

func (s *userServiceImpl) GetUser(ctx context.Context, id int) (*domain.UserSummary, error) {
	return s.repo.GetUser(ctx, id)
}

func (s *userServiceImpl) ListUsers(ctx context.Context) ([]domain.UserSummary, error) {
	return s.repo.ListUsers(ctx)
}

func (s *userServiceImpl) CreateUser(ctx context.Context, name string, email string) (*domain.User, error) {
	user := &domain.User{Name: name, Email: email}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userServiceImpl) DeleteUser(ctx context.Context, id int) error {
	return s.repo.DeleteUser(ctx, id)
}

func (s *userServiceImpl) UpdateUser(ctx context.Context, id int, name string, email string) (*domain.User, error) {
	if _, err := s.repo.GetUser(ctx, id); err != nil {
		return nil, err
	}
	user := &domain.User{ID: id, Name: name, Email: email}
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
