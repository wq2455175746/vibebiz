package model

import "time"

const (
	ProjectStateReady = "ready"
	ProjectStateError = "error"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Prompt    string    `json:"prompt"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
