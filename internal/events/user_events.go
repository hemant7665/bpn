package events

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"project-serverless/internal/domain"
)

const (
	UserCreatedEventType = "UserCreated"
	UserUpdatedEventType = "UserUpdated"
	UserEventVersion     = 1
	// UserReadModelSyncEventType is used by SQS messages that only trigger an MV refresh (optional CDC supplement).
	UserReadModelSyncEventType = "UserReadModelSync"
)

// UserCreatedEvent is published to the domain Kinesis stream after a successful write (and by userEventWorker on CDC).
type UserCreatedEvent struct {
	EventType  string    `json:"eventType"`
	EventID    string    `json:"eventId"`
	Version    int       `json:"version"`
	UserID     string    `json:"userId"`
	TenantID   string    `json:"tenantId"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	CreatedAt  time.Time `json:"createdAt"`
	OccurredAt time.Time `json:"occurredAt"`
}

// UserUpdatedEvent is published or consumed when routing domain-style updates to the user projection worker.
type UserUpdatedEvent struct {
	EventType  string    `json:"eventType"`
	EventID    string    `json:"eventId"`
	Version    int       `json:"version"`
	UserID     string    `json:"userId"`
	TenantID   string    `json:"tenantId"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	IsDeleted  bool      `json:"isDeleted"`
	UpdatedAt  time.Time `json:"updatedAt"`
	OccurredAt time.Time `json:"occurredAt"`
}

// NewUserCreatedEventFromUser builds a v1 UserCreatedEvent from the write model row after insert.
func NewUserCreatedEventFromUser(u *domain.User) UserCreatedEvent {
	now := time.Now().UTC()
	return UserCreatedEvent{
		EventType:  UserCreatedEventType,
		EventID:    "evt_" + uuid.New().String(),
		Version:    UserEventVersion,
		UserID:     strconv.Itoa(u.ID),
		TenantID:   u.TenantID,
		Email:      u.Email,
		Username:   u.Username,
		Role:       "",
		CreatedAt:  u.CreatedAt,
		OccurredAt: now,
	}
}

// NewUserUpdatedEventFromUser builds a v1 UserUpdatedEvent after update or delete (isDeleted true).
func NewUserUpdatedEventFromUser(u *domain.User, isDeleted bool) UserUpdatedEvent {
	now := time.Now().UTC()
	return UserUpdatedEvent{
		EventType:  UserUpdatedEventType,
		EventID:    "evt_" + uuid.New().String(),
		Version:    UserEventVersion,
		UserID:     strconv.Itoa(u.ID),
		TenantID:   u.TenantID,
		Email:      u.Email,
		Username:   u.Username,
		Role:       "",
		IsDeleted:  isDeleted,
		UpdatedAt:  now,
		OccurredAt: now,
	}
}
