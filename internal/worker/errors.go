package worker

import "fmt"

// ErrPermanent signifies an error that is unlikely to be resolved by a retry,
// such as a validation error (4xx).
type ErrPermanent struct{ Err error }

func (e *ErrPermanent) Error() string { return fmt.Sprintf("permanent error: %v", e.Err) }
func (e *ErrPermanent) Unwrap() error { return e.Err }

// ErrTransient signifies a temporary error that may be resolved by a retry,
// such as a network issue or a temporary server error (5xx).
type ErrTransient struct{ Err error }

func (e *ErrTransient) Error() string { return fmt.Sprintf("transient error: %v", e.Err) }
func (e *ErrTransient) Unwrap() error { return e.Err }