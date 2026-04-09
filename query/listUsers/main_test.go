package main

import (
	"context"
	"testing"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
)

type listUsersUserServiceMock struct {
	lastSkip   int
	lastLimit  int
	lastFilter domain.ListUsersFilter
	result     []domain.UserSummary
	total      int64
	err        error
}

func (m *listUsersUserServiceMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m *listUsersUserServiceMock) GetWriteUserByID(context.Context, int) (*domain.User, error) {
	return nil, nil
}
func (m *listUsersUserServiceMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m *listUsersUserServiceMock) DeleteUser(context.Context, int) error          { return nil }
func (m *listUsersUserServiceMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m *listUsersUserServiceMock) ListUsersFiltered(_ context.Context, skip, limit int, filter domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	m.lastSkip = skip
	m.lastLimit = limit
	m.lastFilter = filter
	return m.result, m.total, m.err
}
func (m *listUsersUserServiceMock) GetUserByEmail(context.Context, string) (*domain.User, error) {
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

func TestHandleRequest_UsesDefaults(t *testing.T) {
	mock := &listUsersUserServiceMock{result: []domain.UserSummary{}, total: 0}
	deps.userService = mock

	_, err := HandleRequest(context.Background(), ListUsersRequest{Authorization: mustAuthHeader(t)})
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}
	if mock.lastLimit != defaultLimit || mock.lastSkip != 0 {
		t.Fatalf("unexpected pagination args: limit=%d skip=%d", mock.lastLimit, mock.lastSkip)
	}
}

func TestHandleRequest_UsesProvidedPagination(t *testing.T) {
	mock := &listUsersUserServiceMock{result: []domain.UserSummary{}, total: 0}
	deps.userService = mock

	skip := 20
	limit := 10
	_, err := HandleRequest(context.Background(), ListUsersRequest{Skip: skip, Limit: limit, Authorization: mustAuthHeader(t)})
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}
	if mock.lastLimit != 10 || mock.lastSkip != 20 {
		t.Fatalf("unexpected pagination args: limit=%d skip=%d", mock.lastLimit, mock.lastSkip)
	}
}

func TestHandleRequest_RejectsNegativeSkip(t *testing.T) {
	mock := &listUsersUserServiceMock{result: []domain.UserSummary{}}
	deps.userService = mock

	_, err := HandleRequest(context.Background(), ListUsersRequest{Skip: -1, Limit: 10, Authorization: mustAuthHeader(t)})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestHandleRequest_ClampsPageSize(t *testing.T) {
	mock := &listUsersUserServiceMock{result: []domain.UserSummary{}, total: 0}
	deps.userService = mock

	_, err := HandleRequest(context.Background(), ListUsersRequest{Skip: 0, Limit: 999, Authorization: mustAuthHeader(t)})
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}
	if mock.lastLimit != maxLimit {
		t.Fatalf("expected clamped limit %d, got %d", maxLimit, mock.lastLimit)
	}
}

func TestHandleRequest_ServiceError(t *testing.T) {
	mock := &listUsersUserServiceMock{err: svcerrors.Internal("db error", nil)}
	deps.userService = mock

	_, err := HandleRequest(context.Background(), ListUsersRequest{Skip: 0, Limit: 10, Authorization: mustAuthHeader(t)})
	if err == nil {
		t.Fatal("expected internal error")
	}
}
