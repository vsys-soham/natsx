// Package middleware provides reusable middleware implementations for natsx
// message handlers. Middleware wraps a natsx.MsgHandler to add cross-cutting
// behavior such as panic recovery and logging.
package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/vsys-soham/natsx"
)

// Recovery returns a middleware that catches panics in the handler and logs them
// instead of crashing the process.
func Recovery(log natsx.Logger) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("handler panic recovered",
						"panic", fmt.Sprintf("%v", r),
						"subject", msg.Subject,
						"stack", string(debug.Stack()),
					)
				}
			}()
			next(msg)
		}
	}
}

// Logging returns a middleware that logs every incoming message at Debug level.
func Logging(log natsx.Logger) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			log.Debug("msg received",
				"subject", msg.Subject,
				"reply", msg.Reply,
				"size", len(msg.Data),
			)
			next(msg)
		}
	}
}
