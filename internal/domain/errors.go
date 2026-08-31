package domain

import "errors"

// Sentinel errors let transport layers classify failures without matching on
// error text. Wrap them with %w so errors.Is keeps working up the stack.
var (
	// ErrNotFound marks a lookup that found no matching record.
	ErrNotFound = errors.New("not found")

	// ErrInvalidInput marks caller-supplied data that failed validation.
	ErrInvalidInput = errors.New("invalid input")

	// ErrConflict marks a request that clashes with current state, such as a
	// move that would exceed a container's capacity.
	ErrConflict = errors.New("conflict")
)
