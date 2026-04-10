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

type deleteUserServiceMock struct {
	getByIDFn func(ctx context.Context, id int) (*domain.User, error)
	deleteFn  func(ctx context.Context, id int) error
}

func (m deleteUserServiceMock) CreateUser(context.Context, *domain.User) error { return nil }
func (m deleteUserServiceMock) GetWriteUserByID(ctx context.Context, id int) (*domain.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, svcerrors.NotFound("not found")
}
func (m deleteUserServiceMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m deleteUserServiceMock) DeleteUser(ctx context.Context, id int) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m deleteUserServiceMock) GetUser(context.Context, string, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m deleteUserServiceMock) ListUsersFiltered(context.Context, string, int, int, domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
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

func TestHandleRequest_DeleteUserSuccess(t *testing.T) {
	deps.userRepo = jwtCallerRepo()
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
	deps.userRepo = jwtCallerRepo()
	deps.userService = deleteUserServiceMock{}
	deps.auditEmitter = delAudit{}
	deps.domainPublisher = noopDomainPub{}
	_, err := HandleRequest(context.Background(), DeleteUserRequest{ID: 1, Authorization: "bad"})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestHandleRequest_DeleteUserWrongTenantNotFound(t *testing.T) {
	deps.userRepo = &helpers.UserRepository{
		GetWriteUserByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{ID: 1, TenantID: "tenant-a"}, nil
		},
	}
	deps.userService = deleteUserServiceMock{
		getByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			return &domain.User{
				ID:        id,
				TenantID:  "tenant-b",
				Username:  "v",
				Email:     "v@b.com",
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	deps.auditEmitter = delAudit{}
	deps.domainPublisher = noopDomainPub{}

	_, err := HandleRequest(context.Background(), DeleteUserRequest{
		ID:            42,
		Authorization: deleteAuthHeader(t),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := svcerrors.IsAppError(err)
	if !ok || ae.Code != svcerrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
