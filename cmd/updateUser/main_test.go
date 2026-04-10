package main

import (
	"context"
	"testing"
	"time"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"

	"gorm.io/gorm"
)

type updateUserServiceMock struct {
	getByIDFn  func(ctx context.Context, id int) (*domain.User, error)
	updateUser func(ctx context.Context, user *domain.User) error
}

func (m updateUserServiceMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m updateUserServiceMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m updateUserServiceMock) UpdateUser(ctx context.Context, user *domain.User) error {
	if m.updateUser != nil {
		return m.updateUser(ctx, user)
	}
	return nil
}
func (m updateUserServiceMock) DeleteUser(context.Context, int) error { return nil }
func (m updateUserServiceMock) GetUser(context.Context, string, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m updateUserServiceMock) ListUsersFiltered(context.Context, string, int, int, domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	return nil, 0, nil
}
func (m updateUserServiceMock) GetUserByEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}
type noopAud struct{}
func (noopAud) EmitUserAudit(context.Context, string, domain.User) error { return nil }

type noopDomainPub struct{}
func (noopDomainPub) PutRecordJSON(context.Context, string, []byte) error { return nil }

func authHeader(t *testing.T) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := auth.GenerateToken(1, "u@example.com")
	if err != nil {
		t.Fatalf("token generation failed: %v", err)
	}
	return "Bearer " + token
}

func jwtCallerRepo() *helpers.UserRepository {
	return &helpers.UserRepository{
		GetWriteUserByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			if id != 1 {
				return nil, gorm.ErrRecordNotFound
			}
			return &domain.User{ID: 1, TenantID: "default-tenant"}, nil
		},
	}
}

func TestHandleRequest_UpdateUserSuccess(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	deps.userRepo = jwtCallerRepo()
	deps.userService = updateUserServiceMock{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{
				ID:           id,
				TenantID:     "default-tenant",
				Username:     "old",
				Email:        "old@example.com",
				PasswordHash: "hash",
				CreatedAt:    createdAt,
			}, nil
		},
		updateUser: func(_ context.Context, user *domain.User) error {
			if user.CreatedAt != createdAt {
				t.Fatalf("createdAt should be preserved")
			}
			if user.PasswordHash != "hash" {
				t.Fatalf("password hash should be preserved")
			}
			return nil
		},
	}
	deps.auditEmitter = noopAud{}
	deps.domainPublisher = noopDomainPub{}

	got, err := HandleRequest(context.Background(), UpdateUserRequest{
		ID:            5,
		Username:      "New Name",
		Email:         "new@example.com",
		Authorization: authHeader(t),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 5 {
		t.Fatalf("expected id=5, got %d", got.ID)
	}
}

func TestHandleRequest_UpdateUserUnauthorized(t *testing.T) {
	deps.userRepo = jwtCallerRepo()
	deps.userService = updateUserServiceMock{}
	deps.auditEmitter = noopAud{}
	deps.domainPublisher = noopDomainPub{}

	_, err := HandleRequest(context.Background(), UpdateUserRequest{
		ID:            1,
		Username:      "A",
		Email:         "a@example.com",
		Authorization: "Bearer bad-token",
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestHandleRequest_UpdateUserWrongTenantNotFound(t *testing.T) {
	deps.userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			if id != 1 {
				return nil, gorm.ErrRecordNotFound
			}
			return &domain.User{ID: 1, TenantID: "tenant-a"}, nil
		},
	}
	deps.userService = updateUserServiceMock{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{
				ID:           id,
				TenantID:     "tenant-b",
				Username:     "other",
				Email:        "o@b.com",
				PasswordHash: "hash",
				CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}
	deps.auditEmitter = noopAud{}
	deps.domainPublisher = noopDomainPub{}

	_, err := HandleRequest(context.Background(), UpdateUserRequest{
		ID:            99,
		Username:      "Hacker",
		Email:         "h@a.com",
		Authorization: authHeader(t),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
