package user

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrLoginAlreadyExists = errors.New("login already exists")
	ErrForbidden          = errors.New("forbidden")
	ErrInactiveUser       = errors.New("user is not active")
	ErrInvalidToken       = errors.New("invalid token")
)
