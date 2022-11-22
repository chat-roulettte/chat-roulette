package v1

import (
	"errors"
)

type ErrResponse struct {
	Error string `json:"error"`
}

var (
	ErrAuthzFailed = errors.New("authorization failed")
)
