package models

import (
	"encoding/json"
	"strings"
)

// Status is where a todo is in its life. The declared order is the order the
// cycle key walks through.
type Status string

const (
	StatusTodo  Status = "todo"
	StatusDoing Status = "doing"
	StatusDone  Status = "done"

	// StatusAll is the "no status filter" value. It is never stored on a todo.
	StatusAll Status = ""
)

// Next walks the cycle: todo -> doing -> done -> todo.
func (s Status) Next() Status {
	switch s {
	case StatusDoing:
		return StatusDone
	case StatusDone:
		return StatusTodo
	default:
		return StatusDoing
	}
}

// Valid reports whether the value is one we know. Used when loading a file that
// may have been hand-edited.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusDoing, StatusDone:
		return true
	}
	return false
}

// Todo is a single item in the store. The GUI never mutates it directly: every
// change goes through pkg/commands so persistence stays in one place.
type Todo struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	Status Status `json:"status"`
}

// Matches reports whether the todo should survive a filter: title or description
// containing the text, case-insensitively.
func (t *Todo) Matches(filter string) bool {
	if filter == "" {
		return true
	}

	filter = strings.ToLower(filter)
	return strings.Contains(strings.ToLower(t.Title), filter) ||
		strings.Contains(strings.ToLower(t.Detail), filter)
}

// ByStatus counts how many todos are in each status.
type ByStatus struct {
	Todo  int
	Doing int
	Done  int
}

// Total is how many todos were counted.
func (b ByStatus) Total() int { return b.Todo + b.Doing + b.Done }

// Count is the number for one status filter; StatusAll answers with the total.
func (b ByStatus) Count(status Status) int {
	switch status {
	case StatusTodo:
		return b.Todo
	case StatusDoing:
		return b.Doing
	case StatusDone:
		return b.Done
	default:
		return b.Total()
	}
}

// Tally counts a list by status.
func Tally(todos []*Todo) ByStatus {
	var counts ByStatus
	for _, todo := range todos {
		switch todo.Status {
		case StatusDoing:
			counts.Doing++
		case StatusDone:
			counts.Done++
		default:
			counts.Todo++
		}
	}
	return counts
}

// UnmarshalJSON reads the current shape and the older one, where a todo was just
// a "done" bool. Files written by earlier versions keep loading.
func (t *Todo) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Status Status `json:"status"`
		Done   *bool  `json:"done"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	status := raw.Status
	switch {
	case status.Valid():
		// Already current.
	case raw.Done != nil && *raw.Done:
		status = StatusDone
	default:
		status = StatusTodo
	}

	t.ID = raw.ID
	t.Title = raw.Title
	t.Detail = raw.Detail
	t.Status = status
	return nil
}
