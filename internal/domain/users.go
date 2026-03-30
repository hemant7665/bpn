package domain

import "time"

// WriteUser maps to write_model.users — the source-of-truth table.
// GORM uses TableName() to route writes to the correct schema.
type User struct {
	ID        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"not null"`
	Email     string    `json:"email" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName tells GORM to use the write_model schema for this struct.
func (User) TableName() string {
	return "write_model.users"
}

// UserSummary maps to read_model.users_summary — the materialized view.
// Used exclusively for read queries.
type UserSummary struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName tells GORM to query the read_model materialized view.
func (UserSummary) TableName() string {
	return "read_model.users_summary"
}
