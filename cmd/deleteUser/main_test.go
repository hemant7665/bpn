package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"project-serverless/internal/auth"
	"project-serverless/internal/domain"
)

type deleteUserServiceMock struct {
	getByIDFn func(ctx context.Context, id int) (*domain.User, error)
	deleteFn  func(ctx context.Context, id int) error
}

func (m deleteUserServiceMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m deleteUserServiceMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}
func (m deleteUserServiceMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m deleteUserServiceMock) DeleteUser(ctx context.Context, id int) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m deleteUserServiceMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m deleteUserServiceMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	return nil, 0, nil
}
func (m deleteUserServiceMock) GetUserByEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}
type delAudit struct {
	emitFn func(ctx context.Context, operation string, user domain.User) error
}

func (m delAudit) EmitUserAudit(ctx context.Context, operation string, user domain.User) error {
	if m.emitFn != nil {
		return m.emitFn(ctx, operation, user)
	}
	return nil
}

type noopDomainPub struct{}
func (noopDomainPub) PutRecordJSON(context.Context, string, []byte) error { return nil }

func deleteAuthHeader(t *testing.T) string {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := auth.GenerateToken(1, "u@example.com")
	if err != nil {
		t.Fatalf("token generation failed: %v", err)
	}
	return "Bearer " + token
}

func TestHandleRequest_DeleteUserSuccess(t *testing.T) {
	existing := &domain.User{
		ID:        5,
		TenantID:  "default-tenant",
		Username:  "A",
		Email:     "a@example.com",
		CreatedAt: time.Now().UTC(),
	}
	deps.userService = deleteUserServiceMock{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return existing, nil
		},
		deleteFn: func(_ context.Context, id int) error { return nil },
	}
	deps.auditEmitter = delAudit{
		emitFn: func(_ context.Context, op string, user domain.User) error {
			if op != "delete" || user.ID != existing.ID {
				t.Fatalf("unexpected event payload")
			}
			return nil
		},
	}
	deps.domainPublisher = noopDomainPub{}

	got, err := HandleRequest(context.Background(), DeleteUserRequest{
		ID:            5,
		Authorization: deleteAuthHeader(t),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 5 {
		t.Fatalf("expected deleted user id=5, got %d", got.ID)
	}
}

func TestHandleRequest_DeleteUserUnauthorized(t *testing.T) {
	deps.userService = deleteUserServiceMock{}
	deps.auditEmitter = delAudit{}
	deps.domainPublisher = noopDomainPub{}
	_, err := HandleRequest(context.Background(), DeleteUserRequest{ID: 1, Authorization: "bad"})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}
