package service

import (
	"context"

	"project-serverless/internal/domain"
)

// UserService is the application service for user operations (used by command and query Lambdas).
// Commands: cmd/createUser, updateUser, deleteUser, login (JWT). Queries: query/getUser, listUsers.
// GetUser and ListUsersFiltered require caller tenantID (from ResolveTenant in Lambdas) so data stays tenant-scoped.
// The GraphQL subgraph lives under apps/subgraph-user (orchestrator invokes Lambdas), BluePrint-style.
type UserService interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetWriteUserByID(ctx context.Context, id int) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	DeleteUser(ctx context.Context, id int) error
	GetUser(ctx context.Context, tenantID string, id int) (*domain.UserSummary, error)
	ListUsersFiltered(ctx context.Context, tenantID string, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, int64, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
}
