package repository

import (
	"context"

	"gorm.io/gorm"

	"project-serverless/internal/domain"
)

// UserRepository defines write operations against write_model.users
// and read operations against read_model.users_summary.
type UserRepository interface {
	// Write ops — target write_model.users
	CreateUser(ctx context.Context, user *domain.User) error
	UpdateUser(ctx context.Context, user *domain.User) error
	DeleteUser(ctx context.Context, id int) error

	// Read ops — target read_model.users_summary (materialized view)
	GetUser(ctx context.Context, id int) (*domain.UserSummary, error)
	ListUsers(ctx context.Context) ([]domain.UserSummary, error)

	// RefreshView triggers REFRESH MATERIALIZED VIEW CONCURRENTLY
	// Called by the sync worker after every CDC event.
	RefreshView(ctx context.Context) error
}

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{db: db}
}

// --- Write operations (write_model.users) ---

func (r *userRepositoryImpl) CreateUser(ctx context.Context, user *domain.User) error {
	// GORM uses domain.User.TableName() → write_model.users
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepositoryImpl) UpdateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepositoryImpl) DeleteUser(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Delete(&domain.User{}, id).Error
}

// --- Read operations (read_model.users_summary) ---

func (r *userRepositoryImpl) GetUser(ctx context.Context, id int) (*domain.UserSummary, error) {
	var summary domain.UserSummary
	// GORM uses domain.UserSummary.TableName() → read_model.users_summary
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&summary).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func (r *userRepositoryImpl) ListUsers(ctx context.Context) ([]domain.UserSummary, error) {
	var summaries []domain.UserSummary
	if err := r.db.WithContext(ctx).Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

// --- Materialized view refresh ---

func (r *userRepositoryImpl) RefreshView(ctx context.Context) error {
	// CONCURRENTLY avoids locking so reads are never blocked
	return r.db.WithContext(ctx).
		Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY read_model.users_summary").Error
}
