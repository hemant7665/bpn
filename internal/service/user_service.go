package service

import (
	"context"
	"errors"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/repository"

	"gorm.io/gorm"
)

type userServiceImpl struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userServiceImpl{repo: repo}
}

func (s *userServiceImpl) CreateUser(ctx context.Context, user *domain.User) error {
	_, err := s.repo.GetUserByEmail(ctx, user.TenantID, user.Email)
	if err == nil {
		return svcerrors.ErrEmailAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.repo.CreateUser(ctx, user)
}

func (s *userServiceImpl) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	return s.repo.GetWriteUserByID(ctx, id)
}

func (s *userServiceImpl) UpdateUser(ctx context.Context, user *domain.User) error {
	return s.repo.UpdateUser(ctx, user)
}

func (s *userServiceImpl) DeleteUser(ctx context.Context, id int) error {
	return s.repo.DeleteUser(ctx, id)
}

func (s *userServiceImpl) GetUser(ctx context.Context, tenantID string, id int) (*domain.UserSummary, error) {
	return s.repo.GetUser(ctx, tenantID, id)
}

func (s *userServiceImpl) ListUsersFiltered(ctx context.Context, tenantID string, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	items, err := s.repo.ListUsersFiltered(ctx, tenantID, skip, limit, filter)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountUsersFiltered(ctx, tenantID, filter)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *userServiceImpl) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.GetUserByEmail(ctx, "", email)
}
