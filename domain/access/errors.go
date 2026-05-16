package access

import "errors"

var (
	ErrGrantNotFound      = errors.New("access grant not found")
	ErrGrantAlreadyExists = errors.New("access grant already exists")
	ErrForbidden          = errors.New("forbidden")
)
