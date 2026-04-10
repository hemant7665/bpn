package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"project-serverless/internal/domain"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetWriteUserByID(ctx context.Context, id int) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	DeleteUser(ctx context.Context, id int) error

	GetUser(ctx context.Context, tenantID string, id int) (*domain.UserSummary, error)
	GetUserByEmail(ctx context.Context, tenantID, email string) (*domain.User, error)

	ListUsersFiltered(ctx context.Context, tenantID string, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, error)
	CountUsersFiltered(ctx context.Context, tenantID string, filter domain.ListUsersFilter) (int64, error)

	// RefreshUsersSummaryView refreshes read_model.users_summary (CONCURRENTLY when possible).
	RefreshUsersSummaryView(ctx context.Context) error
	// SaveUserReadModel refreshes the MV after domain-style events (same projection as BluePrint readmodel.SaveUser).
	SaveUserReadModel(ctx context.Context, user *domain.User) error
}

type userRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepositoryImpl{db: db}
}

func (r *userRepositoryImpl) CreateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepositoryImpl) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	var u domain.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepositoryImpl) UpdateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepositoryImpl) DeleteUser(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Delete(&domain.User{}, id).Error
}

func (r *userRepositoryImpl) GetUser(ctx context.Context, tenantID string, id int) (*domain.UserSummary, error) {
	tid := domain.NormalizeTenantID(tenantID)
	var summary domain.UserSummary
	if err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tid).First(&summary).Error; err != nil {
		return nil, err
	}
	return &summary, nil
}

func (r *userRepositoryImpl) GetUserByEmail(ctx context.Context, tenantID, email string) (*domain.User, error) {
	tid := strings.TrimSpace(tenantID)
	if tid == "" {
		tid = "default-tenant"
	}
	var user domain.User
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND lower(email) = ?", tid, strings.ToLower(strings.TrimSpace(email))).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepositoryImpl) usersSummaryQuery(ctx context.Context, tenantID string, filter domain.ListUsersFilter) *gorm.DB {
	tid := domain.NormalizeTenantID(tenantID)
	q := r.db.WithContext(ctx).Model(&domain.UserSummary{}).Where("tenant_id = ?", tid)
	return r.applyListFilters(q, filter)
}

func (r *userRepositoryImpl) applyListFilters(db *gorm.DB, filter domain.ListUsersFilter) *gorm.DB {
	if filter.Username != nil && strings.TrimSpace(*filter.Username) != "" {
		pat := "%" + strings.ToLower(strings.TrimSpace(*filter.Username)) + "%"
		db = db.Where("lower(username) LIKE ?", pat)
	}
	if filter.Email != nil && strings.TrimSpace(*filter.Email) != "" {
		pat := "%" + strings.ToLower(strings.TrimSpace(*filter.Email)) + "%"
		db = db.Where("lower(email) LIKE ?", pat)
	}
	return db
}

func (r *userRepositoryImpl) ListUsersFiltered(ctx context.Context, tenantID string, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, error) {
	var summaries []domain.UserSummary
	q := r.usersSummaryQuery(ctx, tenantID, filter)
	if err := q.Order("id ASC").Offset(skip).Limit(limit).Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

func (r *userRepositoryImpl) CountUsersFiltered(ctx context.Context, tenantID string, filter domain.ListUsersFilter) (int64, error) {
	var n int64
	q := r.usersSummaryQuery(ctx, tenantID, filter)
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *userRepositoryImpl) RefreshUsersSummaryView(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	if _, err := sqlDB.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY read_model.users_summary"); err != nil {
		_, err2 := sqlDB.ExecContext(ctx, "REFRESH MATERIALIZED VIEW read_model.users_summary")
		return err2
	}
	return nil
}

func (r *userRepositoryImpl) SaveUserReadModel(ctx context.Context, _ *domain.User) error {
	return r.RefreshUsersSummaryView(ctx)
}
