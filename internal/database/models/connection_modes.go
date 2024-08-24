package models

import (
	"database/sql/driver"
	"errors"
	"strings"
)

var (
	// ErrInvalidConnectionMode is returned when an invalid chat-roulette connection mode is used
	ErrInvalidConnectionMode = errors.New("invalid connection mode")
)

// ConnectionMode is an enum for connection modes
type ConnectionMode int64

const (
	// VirtualConnectionMode represents a virtual connection over Zoom, Meet, etc.
	VirtualConnectionMode ConnectionMode = iota + 1

	// PhysicalConnectionMode represents a physical connection in the real world
	PhysicalConnectionMode

	// HybridConnectionMode represents both a virtual or physical connection
	HybridConnectionMode
)

var connectionModes = map[string]ConnectionMode{
	"virtual":  VirtualConnectionMode,
	"physical": PhysicalConnectionMode,
	"hybrid":   HybridConnectionMode,
}

func (c ConnectionMode) String() string {
	switch c {
	case VirtualConnectionMode:
		return "virtual"
	case PhysicalConnectionMode:
		return "physical"
	case HybridConnectionMode:
		return "hybrid"
	default:
		return ""
	}
}

// Scan implements the Scanner interface
func (c *ConnectionMode) Scan(value interface{}) error {
	s, _ := value.(string)
	s = strings.ToLower(s)

	if v, ok := connectionModes[s]; ok {
		*c = v
		return nil
	}

	return ErrInvalidConnectionMode
}

// Value implements the Valuer interface
func (c ConnectionMode) Value() (driver.Value, error) {
	v := c.String()
	if v != "" {
		return driver.Value(v), nil
	}

	return nil, ErrInvalidConnectionMode
}

// ParseConnectionMode parses a connection mode given by its name.
func ParseConnectionMode(s string) (ConnectionMode, error) {
	if e, ok := connectionModes[s]; ok {
		return e, nil
	}

	return 0, ErrInvalidConnectionMode
}
