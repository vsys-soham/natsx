package middleware

import (
	"github.com/vsys-soham/natsx"
)

// Validator is a function that validates a message before it reaches the handler.
// Return a non-nil error to reject the message.
type Validator func(msg *natsx.Msg) error

// Validation returns a middleware that runs a validator function before calling
// the handler. If the validator returns an error, the message is rejected and
// the handler is not called.
//
// The optional onReject callback is invoked with the message and error when
// validation fails. Use it to log, Nak, or send the message to a DLQ.
func Validation(validate Validator, onReject func(msg *natsx.Msg, err error)) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			if err := validate(msg); err != nil {
				if onReject != nil {
					onReject(msg, err)
				}
				return
			}
			next(msg)
		}
	}
}
