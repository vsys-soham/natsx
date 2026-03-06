// Package natsx provides a high-level, production-ready wrapper around nats.go
// for robust, fast, and easy NATS communication.
package natsx

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
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
	ErrBadPayload   = errors.New("natsx: bad payload")
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

// IsRetryable reports whether err represents a transient failure that can be
// retried. Connection errors, timeouts, and no-responder errors are retryable.
// Encoding, decoding, validation, and bad-payload errors are permanent.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Unwrap OpError to classify the inner error.
	var opErr *OpError
	if errors.As(err, &opErr) {
		return IsRetryable(opErr.Err)
	}

	// Permanent errors — never retry.
	switch {
	case errors.Is(err, ErrEncoding),
		errors.Is(err, ErrDecoding),
		errors.Is(err, ErrInvalidSubj),
		errors.Is(err, ErrBadPayload):
		return false
	}

	// Retryable NATS errors.
	switch {
	case errors.Is(err, nats.ErrTimeout),
		errors.Is(err, nats.ErrNoResponders),
		errors.Is(err, nats.ErrConnectionClosed),
		errors.Is(err, nats.ErrDisconnected),
		errors.Is(err, nats.ErrReconnectBufExceeded),
		errors.Is(err, nats.ErrSlowConsumer):
		return true
	}

	// Default: assume retryable for unknown errors.
	return true
}
