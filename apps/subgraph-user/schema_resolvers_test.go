package main

import (
	"context"
	"testing"
	"time"

	"project-serverless/internal/domain"
)

type orchestratorMock struct{}

func (orchestratorMock) GetUser(_ context.Context, _ string) (*domain.UserSummary, error) {
	return &domain.UserSummary{
		ID:        10,
		Name:      "Alice",
		Email:     "alice@example.com",
		CreatedAt: time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC),
	}, nil
}
func (orchestratorMock) ListUsers(_ context.Context) ([]domain.UserSummary, error) {
	return []domain.UserSummary{
		{ID: 1, Name: "A", Email: "a@example.com", CreatedAt: time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)},
	}, nil
}
func (orchestratorMock) CreateUser(_ context.Context, name, email string) (*domain.User, error) {
	return &domain.User{ID: 1, Name: name, Email: email, CreatedAt: time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)}, nil
}
func (orchestratorMock) DeleteUser(_ context.Context, _ string) (*domain.User, error) {
	return &domain.User{ID: 1, Name: "A", Email: "a@example.com", CreatedAt: time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)}, nil
}
func (orchestratorMock) UpdateUser(_ context.Context, _ string, name, email string) (*domain.User, error) {
	return &domain.User{ID: 1, Name: name, Email: email, CreatedAt: time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)}, nil
}

func TestGetUser_UsesDBTimestamp(t *testing.T) {
	r := &queryResolver{&Resolver{Orchestrator: orchestratorMock{}}}
	got, err := r.GetUser(context.Background(), "10")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	want := "2026-03-31T09:00:00Z"
	if got.CreatedAt != want {
		t.Fatalf("createdAt mismatch: got=%s want=%s", got.CreatedAt, want)
	}
}

func TestListUsers_MapsAllFields(t *testing.T) {
	r := &queryResolver{&Resolver{Orchestrator: orchestratorMock{}}}
	got, err := r.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(got) != 1 || got[0].ID != "1" || got[0].Email != "a@example.com" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
