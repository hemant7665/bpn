package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"project-serverless/internal/domain"
)

type repoMock struct {
	user *domain.UserSummary
	err  error
}

func (m repoMock) CreateUser(context.Context, *domain.User) error            { return nil }
func (m repoMock) UpdateUser(context.Context, *domain.User) error            { return nil }
func (m repoMock) DeleteUser(context.Context, int) error                     { return nil }
func (m repoMock) ListUsers(context.Context) ([]domain.UserSummary, error)   { return nil, nil }
func (m repoMock) UpsertUserSummary(context.Context, *domain.UserSummary) error { return nil }
func (m repoMock) DeleteUserSummary(context.Context, int) error                  { return nil }
func (m repoMock) GetUser(context.Context, int) (*domain.UserSummary, error) { return m.user, m.err }

func TestHandleRequest_AcceptsStringID(t *testing.T) {
	deps.repo = repoMock{
		user: &domain.UserSummary{
			ID:        5,
			Name:      "A",
			Email:     "a@example.com",
			CreatedAt: time.Date(2026, 3, 31, 1, 0, 0, 0, time.UTC),
		},
	}

	got, err := HandleRequest(context.Background(), GetUserRequest{ID: "5"})
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}
	if got.ID != 5 {
		t.Fatalf("unexpected user id: %d", got.ID)
	}
}

func TestHandleRequest_ReturnsValidationForMissingID(t *testing.T) {
	deps.repo = repoMock{}
	_, err := HandleRequest(context.Background(), GetUserRequest{})
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleRequest_ReturnsNotFound(t *testing.T) {
	deps.repo = repoMock{err: errors.New("not found")}
	_, err := HandleRequest(context.Background(), GetUserRequest{ID: 9.0})
	if err == nil {
		t.Fatal("expected not found error")
	}
}
