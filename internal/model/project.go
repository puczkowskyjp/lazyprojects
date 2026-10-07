package model

import "time"

// Project represents a discovered repository or codebase.
type Project struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Type         string    `json:"type"`
	Branch       string    `json:"branch,omitempty"`
	LastModified time.Time `json:"last_modified"`
}
