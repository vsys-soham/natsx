package middleware

import (
	"context"
	"time"

	"github.com/vsys-soham/natsx"
)

// Timeout returns a middleware that creates a context with the given deadline
// for the handler. If the handler does not finish in time, a warning is logged.
//
// The deadline is attached to the message via WithContext, so handlers can
// check msg.Context().Done() for cooperative cancellation.
func Timeout(d time.Duration, log natsx.Logger) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			ctx, cancel := context.WithTimeout(msg.Context(), d)
			defer cancel()

			msg = msg.WithContext(ctx)

			done := make(chan struct{})
			go func() {
				defer close(done)
				next(msg)
			}()

			select {
			case <-done:
				// Handler completed before timeout.
			case <-ctx.Done():
				// Timeout exceeded — wait for handler to finish but log.
				log.Warn("handler exceeded timeout",
					"subject", msg.Subject,
					"timeout", d.String(),
				)
				<-done
			}
		}
	}
}
