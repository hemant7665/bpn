package domain

// UserListPayload is the listUsers query response (CQRS read side).
type UserListPayload struct {
	Items []UserSummary `json:"items"`
	Total int64         `json:"total"`
}
