package events

import "time"

type UserSnapshot struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserEventPayload struct {
	EventType string       `json:"event_type"` // domain | audit
	Entity    string       `json:"entity"`     // user
	Operation string       `json:"operation"`  // insert | update | delete
	User      UserSnapshot `json:"user"`
	Timestamp time.Time    `json:"timestamp"`
}
