package middleware

import (
	"context"

	"github.com/vsys-soham/natsx"
)

// correlationIDKey is the context key for the correlation ID.
type correlationIDKey struct{}

// DefaultCorrelationHeader is the NATS header used for correlation IDs.
const DefaultCorrelationHeader = "X-Correlation-ID"

// CorrelationID returns a middleware that extracts a correlation ID from the
// message header and stores it in the message context.
//
// If no correlation ID is present in the header, the optional generate function
// is called to create one. If generate is nil and no header is found, no
// correlation ID is set.
func CorrelationID(header string, generate func() string) natsx.Middleware {
	if header == "" {
		header = DefaultCorrelationHeader
	}
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			cid := msg.Header.Get(header)
			if cid == "" && generate != nil {
				cid = generate()
			}
			if cid != "" {
				ctx := context.WithValue(msg.Context(), correlationIDKey{}, cid)
				msg = msg.WithContext(ctx)
			}
			next(msg)
		}
	}
}

// GetCorrelationID extracts the correlation ID from a context.
// Returns an empty string if no correlation ID is set.
func GetCorrelationID(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDKey{}).(string); ok {
		return v
	}
	return ""
}
