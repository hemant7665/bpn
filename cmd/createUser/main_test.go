package main

import (
	"context"
	"errors"
	"testing"

	"project-serverless/internal/domain"
)

type createUserServiceMock struct {
	createFn func(ctx context.Context, user *domain.User) error
}

func (m createUserServiceMock) CreateUser(ctx context.Context, user *domain.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return nil
}
func (m createUserServiceMock) GetWriteUserByID(context.Context, int) (*domain.User, error) {
	return nil, nil
}
func (m createUserServiceMock) UpdateUser(context.Context, *domain.User) error { return nil }
func (m createUserServiceMock) DeleteUser(context.Context, int) error          { return nil }
func (m createUserServiceMock) GetUser(context.Context, int) (*domain.UserSummary, error) {
	return nil, nil
}
func (m createUserServiceMock) ListUsersFiltered(context.Context, int, int, domain.ListUsersFilter) ([]domain.UserSummary, int64, error) {
	return nil, 0, nil
}
func (m createUserServiceMock) GetUserByEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}
type mockAudit struct {
	emitFn func(ctx context.Context, operation string, user domain.User) error
}

func (m mockAudit) EmitUserAudit(ctx context.Context, operation string, user domain.User) error {
	if m.emitFn != nil {
		return m.emitFn(ctx, operation, user)
	}
	return nil
}

type noopDomainPub struct{}

func (noopDomainPub) PutRecordJSON(context.Context, string, []byte) error { return nil }

func TestHandleRequest_CreateUserSuccess(t *testing.T) {
	deps.userService = createUserServiceMock{
		createFn: func(_ context.Context, user *domain.User) error {
			user.ID = 42
			return nil
		},
	}
	deps.auditEmitter = mockAudit{
		emitFn: func(_ context.Context, operation string, user domain.User) error {
			if operation != "insert" || user.ID != 42 {
				t.Fatalf("unexpected emit payload: op=%s user=%+v", operation, user)
			}
			return nil
		},
	}
	deps.domainPublisher = noopDomainPub{}

	got, err := HandleRequest(context.Background(), CreateUserRequest{
		Username: "Alice",
		Email:    "Alice@Example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("HandleRequest error: %v", err)
	}
	if got.ID != 42 {
		t.Fatalf("expected id 42, got %d", got.ID)
	}
	if got.PasswordHash == "" || got.PasswordHash == "password123" {
		t.Fatalf("password hash was not generated correctly")
	}
	if got.Email != "alice@example.com" {
		t.Fatalf("expected normalized email, got %s", got.Email)
	}
}

func TestHandleRequest_CreateUserValidationError(t *testing.T) {
	deps.userService = createUserServiceMock{}
	deps.auditEmitter = mockAudit{}
	deps.domainPublisher = noopDomainPub{}
	_, err := HandleRequest(context.Background(), CreateUserRequest{Username: "A", Email: "bad-email"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestHandleRequest_CreateUserEventFailure(t *testing.T) {
	deps.userService = createUserServiceMock{
		createFn: func(_ context.Context, user *domain.User) error {
			user.ID = 10
			return nil
		},
	}
	deps.auditEmitter = mockAudit{
		emitFn: func(_ context.Context, _ string, _ domain.User) error {
			return errors.New("kinesis down")
		},
	}
	deps.domainPublisher = noopDomainPub{}

	_, err := HandleRequest(context.Background(), CreateUserRequest{
		Username: "Alice",
		Email:    "alice@example.com",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error when audit emit fails")
	}
}
