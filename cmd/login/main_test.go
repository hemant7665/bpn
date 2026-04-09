package main

import (
	"context"
	"testing"
	"time"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
)

type loginServiceMock struct {
	getByEmailFn func(ctx context.Context, email string) (*domain.User, error)
}

func (m loginServiceMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m loginServiceMock) GetWriteUserByID(context.Context, int) (*domain.User, error) {
	return nil, nil
}
func (m loginServiceMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m loginServiceMock) DeleteUser(context.Context, int) error { return nil }
func (m loginServiceMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m loginServiceMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	return nil, 0, nil
}
func (m loginServiceMock) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, svcerrors.NotFound("not found")
}
func TestHandleRequest_LoginSuccess(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-for-login")

	hash, err := auth.HashPassword("password12345")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	deps.userService = loginServiceMock{
		getByEmailFn: func(_ context.Context, email string) (*domain.User, error) {
			if email != "user@example.com" {
				t.Fatalf("expected normalized email, got %q", email)
			}
			return &domain.User{
				ID:           7,
				TenantID:     "default-tenant",
				Username:     "user",
				Email:        "user@example.com",
				PasswordHash: hash,
				CreatedAt:    time.Now().UTC(),
			}, nil
		},
	}

	got, err := HandleRequest(context.Background(), LoginRequest{
		Email:    "User@Example.com",
		Password: "password12345",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if got == nil || got.Token == "" {
		t.Fatalf("expected non-empty token, got %+v", got)
	}
}

func TestHandleRequest_ValidationError(t *testing.T) {
	deps.userService = loginServiceMock{}
	_, err := HandleRequest(context.Background(), LoginRequest{
		Email:    "not-an-email",
		Password: "password12345",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeValidation {
		t.Fatalf("expected validation AppError, got %v", err)
	}
}

func TestHandleRequest_InvalidCredentials_UserNotFound(t *testing.T) {
	deps.userService = loginServiceMock{
		getByEmailFn: func(context.Context, string) (*domain.User, error) {
			return nil, svcerrors.NotFound("db: no rows")
		},
	}
	_, err := HandleRequest(context.Background(), LoginRequest{
		Email:    "missing@example.com",
		Password: "password12345",
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeInvalidCredential {
		t.Fatalf("expected invalid credential AppError, got %v", err)
	}
}

func TestHandleRequest_InvalidCredentials_WrongPassword(t *testing.T) {
	hash, err := auth.HashPassword("correctPassword1")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	deps.userService = loginServiceMock{
		getByEmailFn: func(context.Context, string) (*domain.User, error) {
			return &domain.User{
				ID:           1,
				TenantID:     "t",
				Username:     "u",
				Email:        "u@example.com",
				PasswordHash: hash,
				CreatedAt:    time.Now().UTC(),
			}, nil
		},
	}
	_, err = HandleRequest(context.Background(), LoginRequest{
		Email:    "u@example.com",
		Password: "wrongPassword999",
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeInvalidCredential {
		t.Fatalf("expected invalid credential AppError, got %v", err)
	}
}
