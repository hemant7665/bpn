package main

import (
	"context"
	"testing"
	"time"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
)

type userServiceMock struct {
	user *domain.UserSummary
	err  error
}

func (m userServiceMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m userServiceMock) GetWriteUserByID(context.Context, int) (*domain.User, error) {
	return nil, nil
}
func (m userServiceMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m userServiceMock) DeleteUser(context.Context, int) error          { return nil }
func (m userServiceMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return m.user, m.err
}
func (m userServiceMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	return nil, 0, nil
}
func (m userServiceMock) GetUserByEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func mustAuthHeader(t *testing.T) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := auth.GenerateToken(1, "t@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return "Bearer " + token
}

func TestHandleRequest_AcceptsStringID(t *testing.T) {
	deps.userService = userServiceMock{
		user: &domain.UserSummary{
			ID:        5,
			TenantID:  "default-tenant",
			Username:  "A",
			Email:     "a@example.com",
			CreatedAt: time.Date(2026, 3, 31, 1, 0, 0, 0, time.UTC),
		},
	}

	got, err := HandleRequest(context.Background(), GetUserRequest{ID: "5", Authorization: mustAuthHeader(t)})
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}
	if got.ID != 5 {
		t.Fatalf("unexpected user id: %d", got.ID)
	}
}

func TestHandleRequest_ReturnsValidationForMissingID(t *testing.T) {
	deps.userService = userServiceMock{}
	_, err := HandleRequest(context.Background(), GetUserRequest{Authorization: mustAuthHeader(t)})
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleRequest_ReturnsNotFound(t *testing.T) {
	deps.userService = userServiceMock{err: svcerrors.NotFound("not found")}
	_, err := HandleRequest(context.Background(), GetUserRequest{ID: 9.0, Authorization: mustAuthHeader(t)})
	if err == nil {
		t.Fatal("expected not found error")
	}
}
