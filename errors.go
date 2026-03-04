// Package natsx provides a high-level, production-ready wrapper around nats.go
// for robust, fast, and easy NATS communication.
package natsx

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by the library.
var (
	ErrNotConnected = errors.New("natsx: not connected")
	ErrTimeout      = errors.New("natsx: timeout")
	ErrEncoding     = errors.New("natsx: encoding error")
	ErrDecoding     = errors.New("natsx: decoding error")
	ErrDrain        = errors.New("natsx: drain error")
	ErrClosed       = errors.New("natsx: connection closed")
	ErrInvalidSubj  = errors.New("natsx: invalid subject")
)

// OpError wraps an error with the operation that caused it.
type OpError struct {
	Op  string // operation name, e.g. "Publish", "Subscribe"
	Err error  // underlying error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("natsx.%s: %v", e.Op, e.Err)
}

func (e *OpError) Unwrap() error {
	return e.Err
}

// WrapError creates an OpError that records the operation and underlying error.
func WrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &OpError{Op: op, Err: err}
}
