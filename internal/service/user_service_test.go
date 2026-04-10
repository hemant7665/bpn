package service_test

import (
	"context"
	"errors"
	"testing"

	"project-serverless/internal/domain"
	svcerrors "project-serverless/internal/errors"
	"project-serverless/internal/helpers"
	"project-serverless/internal/service"

	"gorm.io/gorm"
)

func TestCreateUser_emailAlreadyExists(t *testing.T) {
	u := &domain.User{TenantID: "t1", Email: "a@b.c"}
	repo := &helpers.UserRepository{
		GetUserByEmailFn: func(context.Context, string, string) (*domain.User, error) {
			return &domain.User{ID: 1, Email: u.Email}, nil
		},
	}
	svc := service.NewUserService(repo)
	err := svc.CreateUser(context.Background(), u)
	if !errors.Is(err, svcerrors.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestCreateUser_getByEmailFails(t *testing.T) {
	dbErr := errors.New("db connection refused")
	repo := &helpers.UserRepository{
		GetUserByEmailFn: func(context.Context, string, string) (*domain.User, error) {
			return nil, dbErr
		},
	}
	svc := service.NewUserService(repo)
	err := svc.CreateUser(context.Background(), &domain.User{TenantID: "t", Email: "x@y.z"})
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected db error, got %v", err)
	}
}

func TestCreateUser_successAfterNotFound(t *testing.T) {
	var created *domain.User
	repo := &helpers.UserRepository{
		GetUserByEmailFn: func(context.Context, string, string) (*domain.User, error) {
			return nil, gorm.ErrRecordNotFound
		},
		CreateUserFn: func(_ context.Context, user *domain.User) error {
			created = user
			return nil
		},
	}
	svc := service.NewUserService(repo)
	in := &domain.User{TenantID: "acme", Email: "new@acme.test", Username: "u"}
	if err := svc.CreateUser(context.Background(), in); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created == nil || created.Email != in.Email {
		t.Fatalf("unexpected created user: %+v", created)
	}
}

func TestCreateUser_createFails(t *testing.T) {
	createErr := errors.New("unique violation")
	repo := &helpers.UserRepository{
		GetUserByEmailFn: func(context.Context, string, string) (*domain.User, error) {
			return nil, gorm.ErrRecordNotFound
		},
		CreateUserFn: func(context.Context, *domain.User) error {
			return createErr
		},
	}
	svc := service.NewUserService(repo)
	err := svc.CreateUser(context.Background(), &domain.User{TenantID: "t", Email: "e@e.e"})
	if !errors.Is(err, createErr) {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestGetWriteUserByID_delegates(t *testing.T) {
	want := &domain.User{ID: 42, Email: "x@y.z"}
	repo := &helpers.UserRepository{
		GetWriteUserByIDFn: func(_ context.Context, id int) (*domain.User, error) {
			if id != 42 {
				t.Fatalf("id %d", id)
			}
			return want, nil
		},
	}
	svc := service.NewUserService(repo)
	got, err := svc.GetWriteUserByID(context.Background(), 42)
	if err != nil || got != want {
		t.Fatalf("got (%v, %v), want (%v, nil)", got, err, want)
	}
}

func TestListUsersFiltered_listError(t *testing.T) {
	listErr := errors.New("list failed")
	repo := &helpers.UserRepository{
		ListUsersFilteredFn: func(context.Context, string, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
			return nil, listErr
		},
	}
	svc := service.NewUserService(repo)
	_, _, err := svc.ListUsersFiltered(context.Background(), "t", 0, 10, domain.ListUsersFilter{})
	if !errors.Is(err, listErr) {
		t.Fatalf("expected list error, got %v", err)
	}
}

func TestListUsersFiltered_countError(t *testing.T) {
	countErr := errors.New("count failed")
	repo := &helpers.UserRepository{
		ListUsersFilteredFn: func(context.Context, string, int, int, domain.ListUsersFilter) ([]domain.UserSummary, error) {
			return []domain.UserSummary{{ID: 1}}, nil
		},
		CountUsersFilteredFn: func(context.Context, string, domain.ListUsersFilter) (int64, error) {
			return 0, countErr
		},
	}
	svc := service.NewUserService(repo)
	_, _, err := svc.ListUsersFiltered(context.Background(), "t", 0, 10, domain.ListUsersFilter{})
	if !errors.Is(err, countErr) {
		t.Fatalf("expected count error, got %v", err)
	}
}

func TestListUsersFiltered_success(t *testing.T) {
	items := []domain.UserSummary{{ID: 1, Email: "a@b.c"}}
	repo := &helpers.UserRepository{
		ListUsersFilteredFn: func(_ context.Context, tenantID string, skip, limit int, f domain.ListUsersFilter) ([]domain.UserSummary, error) {
			if tenantID != "acme" || skip != 5 || limit != 10 {
				t.Fatalf("tenant/skip/limit %q %d/%d", tenantID, skip, limit)
			}
			return items, nil
		},
		CountUsersFilteredFn: func(context.Context, string, domain.ListUsersFilter) (int64, error) {
			return 99, nil
		},
	}
	svc := service.NewUserService(repo)
	got, total, err := svc.ListUsersFiltered(context.Background(), "acme", 5, 10, domain.ListUsersFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 99 || len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("got items=%v total=%d", got, total)
	}
}

func TestUpdateUser_delegates(t *testing.T) {
	var saw *domain.User
	repo := &helpers.UserRepository{
		UpdateUserFn: func(_ context.Context, u *domain.User) error {
			saw = u
			return nil
		},
	}
	svc := service.NewUserService(repo)
	u := &domain.User{ID: 7, Email: "e@e.e"}
	if err := svc.UpdateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if saw != u {
		t.Fatal("expected same user pointer")
	}
}

func TestDeleteUser_delegates(t *testing.T) {
	var sawID int
	repo := &helpers.UserRepository{
		DeleteUserFn: func(_ context.Context, id int) error {
			sawID = id
			return nil
		},
	}
	svc := service.NewUserService(repo)
	if err := svc.DeleteUser(context.Background(), 99); err != nil {
		t.Fatal(err)
	}
	if sawID != 99 {
		t.Fatalf("id %d", sawID)
	}
}

func TestGetUser_delegates(t *testing.T) {
	want := &domain.UserSummary{ID: 5, Email: "r@r.r"}
	repo := &helpers.UserRepository{
		GetUserFn: func(_ context.Context, tenantID string, id int) (*domain.UserSummary, error) {
			if tenantID != "t1" || id != 5 {
				t.Fatalf("tenant %q id %d", tenantID, id)
			}
			return want, nil
		},
	}
	svc := service.NewUserService(repo)
	got, err := svc.GetUser(context.Background(), "t1", 5)
	if err != nil || got != want {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestGetUserByEmail_delegatesWithEmptyTenant(t *testing.T) {
	want := &domain.User{ID: 3, Email: "lookup@test.dev"}
	var sawEmail string
	repo := &helpers.UserRepository{
		GetUserByEmailFn: func(_ context.Context, tenantID, email string) (*domain.User, error) {
			sawEmail = email
			if tenantID != "" {
				t.Fatalf("expected empty tenantID for service lookup, got %q", tenantID)
			}
			return want, nil
		},
	}
	svc := service.NewUserService(repo)
	got, err := svc.GetUserByEmail(context.Background(), want.Email)
	if err != nil || got != want || sawEmail != want.Email {
		t.Fatalf("got (%v, %v) sawEmail=%q", got, err, sawEmail)
	}
}
