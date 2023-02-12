package mirrosa

import (
	"context"
)

// Component represents a specific component that will be validated
type Component interface {
	// FilterValue returns the name of the component to implement the github.com/charmbracelet/bubbles/list Item interface
	FilterValue() string

	// Title returns the name of the component for the bubbletea TUI
	Title() string

	// Description returns a thorough description of the component's expected configuration.
	// It should allow a new user of ROSA to understand what the expected state is and why it should be that way.
	Description() string

	// Validate checks a component for any misconfiguration and returns any error
	Validate(ctx context.Context) error
}
