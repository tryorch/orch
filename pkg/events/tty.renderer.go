package events

import (
	"fmt"
	"strings"
	"time"
)

type TTYRenderer struct{}

func NewTTYRenderer() *TTYRenderer {
	return &TTYRenderer{}
}

func (r *TTYRenderer) Render(e Event) {
	icon := ""
	switch e.Type {
	case EventStart:
		icon = "[⚡]"
	case EventSuccess:
		icon = "[✓]"
	case EventFailure:
		icon = "[✗]"
	case EventInfo:
		icon = "[ℹ]"
	case EventWarning:
		icon = "[⚠]"
	}

	line := fmt.Sprintf("%s \033[33m%s\033[0m(\033[1m%s\033[0m)  \033[35m%-6s\033[0m %s", icon, e.Component, e.Adapter, e.Runner, e.Message)
	if e.Type == EventSuccess && e.Duration > 0 {
		line += fmt.Sprintf(" (%s)", e.Duration.Round(time.Second))
	}
	fmt.Println(line)
	if e.Type == EventFailure && e.Err != nil {
		fmt.Printf("    %v\n", strings.Replace(e.Err.Error(), "\n", "\n    ", -1))
	}
	if e.Type == EventWarning && e.Hint != "" {
		fmt.Printf("    Hint: %v\n", strings.Replace(e.Hint, "\n", "\n          ", -1))
	}
}

func (r *TTYRenderer) RenderError(err error) {
	fmt.Printf("[✗] %v\n", err)
}
