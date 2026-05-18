package events

import "time"

type Type string

const (
	EventStart   Type = "start"
	EventSuccess Type = "success"
	EventFailure Type = "failure"
	EventInfo    Type = "info"
	EventWarning Type = "warning"
)

type Event struct {
	Type      Type
	Component string
	Adapter   string
	Runner    string
	Stage     string

	Message  string
	Hint     string
	Err      error
	Duration time.Duration
}
