package domain

import "time"

// User is the write_model.users aggregate (CQRS command side).
type User struct {
	ID           int        `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID     string     `json:"tenant_id" gorm:"column:tenant_id;not null"`
	Username     string     `json:"username" gorm:"not null"`
	Email        string     `json:"email" gorm:"not null"`
	PhoneNo      string     `json:"phone_no" gorm:"column:phone_no"`
	DateOfBirth  *time.Time `json:"date_of_birth,omitempty" gorm:"column:date_of_birth;type:date"`
	Gender       string     `json:"gender"`
	PasswordHash string     `json:"-" gorm:"column:password_hash;not null"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

func (User) TableName() string {
	return "users"
}

// UserSummary maps read_model.users_summary (Postgres MATERIALIZED VIEW over write_model.users).
type UserSummary struct {
	ID          int        `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	PhoneNo     string     `json:"phone_no"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty" gorm:"column:date_of_birth;type:date"`
	Gender      string     `json:"gender"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (UserSummary) TableName() string {
	return "users_summary"
}

// ListUsersFilter holds optional filters for list queries.
type ListUsersFilter struct {
	Username *string
	Email    *string
}
