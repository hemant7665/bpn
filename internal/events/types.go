package events

import "time"

// UserSnapshot is carried in audit payloads (no password).
type UserSnapshot struct {
	ID          int        `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	PhoneNo     string     `json:"phone_no"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Gender      string     `json:"gender"`
	CreatedAt   time.Time  `json:"created_at"`
}

type UserEventPayload struct {
	EventType string       `json:"event_type"`
	Entity    string       `json:"entity"`
	Operation string       `json:"operation"`
	User      UserSnapshot `json:"user"`
	Timestamp time.Time    `json:"timestamp"`
}
