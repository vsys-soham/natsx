package middleware

import (
	"github.com/vsys-soham/natsx"
)

// ConcurrencyLimit returns a middleware that limits how many messages can be
// processed concurrently. When the limit is reached, additional messages block
// until a slot becomes available.
//
// This is useful for protecting downstream resources (databases, APIs) from
// being overwhelmed by a burst of messages.
func ConcurrencyLimit(maxConcurrent int) natsx.Middleware {
	sem := make(chan struct{}, maxConcurrent)
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			sem <- struct{}{}
			defer func() { <-sem }()
			next(msg)
		}
	}
}
