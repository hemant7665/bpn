// Package helpers provides shared test doubles and small test utilities.
package helpers

import (
	"context"

	"project-serverless/internal/domain"
	"project-serverless/internal/repository"

	"gorm.io/gorm"
)

// UserRepository is a configurable test double implementing repository.UserRepository.
type UserRepository struct {
	CreateUserFn              func(ctx context.Context, user *domain.User) error
	GetWriteUserByIDFn        func(ctx context.Context, id int) (*domain.User, error)
	UpdateUserFn              func(ctx context.Context, user *domain.User) error
	DeleteUserFn              func(ctx context.Context, id int) error
	GetUserFn                 func(ctx context.Context, tenantID string, id int) (*domain.UserSummary, error)
	GetUserByEmailFn          func(ctx context.Context, tenantID, email string) (*domain.User, error)
	ListUsersFilteredFn       func(ctx context.Context, tenantID string, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, error)
	CountUsersFilteredFn      func(ctx context.Context, tenantID string, filter domain.ListUsersFilter) (int64, error)
	RefreshUsersSummaryViewFn func(ctx context.Context) error
	SaveUserReadModelFn       func(ctx context.Context, user *domain.User) error
}

var _ repository.UserRepository = (*UserRepository)(nil)

func (m *UserRepository) CreateUser(ctx context.Context, user *domain.User) error {
	if m.CreateUserFn != nil {
		return m.CreateUserFn(ctx, user)
	}
	return nil
}

func (m *UserRepository) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.GetWriteUserByIDFn != nil {
		return m.GetWriteUserByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *UserRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	if m.UpdateUserFn != nil {
		return m.UpdateUserFn(ctx, user)
	}
	return nil
}

func (m *UserRepository) DeleteUser(ctx context.Context, id int) error {
	if m.DeleteUserFn != nil {
		return m.DeleteUserFn(ctx, id)
	}
	return nil
}

func (m *UserRepository) GetUser(ctx context.Context, tenantID string, id int) (*domain.UserSummary, error) {
	if m.GetUserFn != nil {
		return m.GetUserFn(ctx, tenantID, id)
	}
	return nil, nil
}

func (m *UserRepository) GetUserByEmail(ctx context.Context, tenantID, email string) (*domain.User, error) {
	if m.GetUserByEmailFn != nil {
		return m.GetUserByEmailFn(ctx, tenantID, email)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *UserRepository) ListUsersFiltered(ctx context.Context, tenantID string, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, error) {
	if m.ListUsersFilteredFn != nil {
		return m.ListUsersFilteredFn(ctx, tenantID, skip, limit, filter)
	}
	return nil, nil
}

func (m *UserRepository) CountUsersFiltered(ctx context.Context, tenantID string, filter domain.ListUsersFilter) (int64, error) {
	if m.CountUsersFilteredFn != nil {
		return m.CountUsersFilteredFn(ctx, tenantID, filter)
	}
	return 0, nil
}

func (m *UserRepository) RefreshUsersSummaryView(ctx context.Context) error {
	if m.RefreshUsersSummaryViewFn != nil {
		return m.RefreshUsersSummaryViewFn(ctx)
	}
	return nil
}

func (m *UserRepository) SaveUserReadModel(ctx context.Context, user *domain.User) error {
	if m.SaveUserReadModelFn != nil {
		return m.SaveUserReadModelFn(ctx, user)
	}
	return nil
}
