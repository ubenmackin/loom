// Package store provides data access layer for Loom entities.
package store

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidTransition is returned when a status change is not allowed.
	ErrInvalidTransition = errors.New("invalid status transition")

	// ErrSelfDependency is returned when a task is set to depend on itself.
	ErrSelfDependency = errors.New("task cannot depend on itself")

	// ErrCycleDetected is returned when adding a dependency would create a cycle.
	ErrCycleDetected = errors.New("dependency would create a cycle")

	// ErrUnauthorizedAuthor is returned when an operation on a resource (like a comment)
	// is attempted by a user who is not its original author.
	ErrUnauthorizedAuthor = errors.New("only the author can modify this resource")

	// ErrInvalidCredentials is returned when authentication fails.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrEmailAlreadyRegistered is returned when a user tries to register with an
	// email address that is already in use.
	ErrEmailAlreadyRegistered = errors.New("email address already registered")

	// ErrUsernameAlreadyTaken is returned when a user tries to register with a
	// username that is already taken.
	ErrUsernameAlreadyTaken = errors.New("username already taken")
)
