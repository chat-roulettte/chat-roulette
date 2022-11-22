package models

import (
	"database/sql/driver"
	"errors"
	"strings"
)

var (
	// ErrInvalidInterval is returned when an invalid chat roulette interval is used
	ErrInvalidInterval = errors.New("invalid chat roulette interval")
)

// IntervalEnum is an enum for chat roulette intervals
type IntervalEnum int64

const (
	// Weekly is every 7 days
	Weekly IntervalEnum = 7

	// Biweekly is every 2 weeks, 14 days
	Biweekly IntervalEnum = 14

	// Triweekly is every 3 weeks, 21 days
	Triweekly IntervalEnum = 21

	// Monthly is every 4 weeks, 28 days
	Monthly IntervalEnum = 28
)

var intervals = map[string]IntervalEnum{
	"weekly":    Weekly,
	"biweekly":  Biweekly,
	"triweekly": Triweekly,
	"monthly":   Monthly,
}

func (i IntervalEnum) String() string {
	switch i {
	case Weekly:
		return "weekly"
	case Biweekly:
		return "biweekly"
	case Triweekly:
		return "triweekly"
	case Monthly:
		return "monthly"
	default:
		return ""
	}
}

// Scan implements the Scanner interface
func (i *IntervalEnum) Scan(value interface{}) error {
	s, _ := value.(string)
	s = strings.ToLower(s)

	if v, ok := intervals[s]; ok {
		*i = v
		return nil
	}

	return ErrInvalidInterval
}

// Value implements the Valuer interface
func (i IntervalEnum) Value() (driver.Value, error) {
	v := i.String()
	if v != "" {
		return driver.Value(v), nil
	}

	return nil, ErrInvalidInterval
}

// ParseInterval parses a chat roulette interval given by its name.
func ParseInterval(s string) (IntervalEnum, error) {
	if e, ok := intervals[s]; ok {
		return e, nil
	}

	return 0, ErrInvalidInterval
}
