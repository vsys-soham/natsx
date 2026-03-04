package natsx

// Middleware wraps a MsgHandler to add cross-cutting behavior.
type Middleware func(MsgHandler) MsgHandler

// Chain applies middlewares to a handler in order. The first middleware in the
// slice is the outermost (executed first).
func Chain(handler MsgHandler, mws ...Middleware) MsgHandler {
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}
