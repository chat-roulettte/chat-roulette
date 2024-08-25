package models

import (
	"database/sql/driver"
	"errors"
	"strings"
)

var (
	// ErrInvalidGender is returned when an invalid gender is used
	ErrInvalidGender = errors.New("invalid gender")

	// ErrNilGender is returned when gender is nil
	ErrNilGender = errors.New("nil gender")
)

// Gender is an enum for gender values
type Gender int8

const (
	// Male represents the male gender
	Male Gender = iota + 1

	// Female represents the female gender
	Female
)

var genderValues = map[string]Gender{
	"male":   Male,
	"female": Female,
}

func (g Gender) String() string {
	switch g {
	case Male:
		return "male"
	case Female:
		return "female"
	default:
		return ""
	}
}

// Scan implements the Scanner interface for the Gender type
func (g *Gender) Scan(value interface{}) error {
	if value == nil {
		return ErrNilGender
	}

	s, ok := value.(string)
	if !ok {
		return ErrInvalidGender
	}

	s = strings.ToLower(s)
	if v, ok := genderValues[s]; ok {
		*g = v
		return nil
	}

	return ErrInvalidGender
}

// Value implements the Valuer interface for the Gender type
func (g Gender) Value() (driver.Value, error) {
	v := g.String()
	if v != "" {
		return driver.Value(v), nil
	}

	return nil, ErrInvalidGender
}

// ParseGender parses a gender given by its name
func ParseGender(s string) (Gender, error) {
	if e, ok := genderValues[s]; ok {
		return e, nil
	}

	return 0, ErrInvalidGender
}
