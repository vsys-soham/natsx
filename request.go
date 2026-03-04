package natsx

import (
	"context"
	"encoding/json"
)

// Request sends a request on the given subject and waits for a single reply.
// The context controls the deadline / cancellation of the request.
func (c *Client) Request(ctx context.Context, subject string, data []byte) (*Msg, error) {
	reply, err := c.nc.RequestWithContext(ctx, subject, data)
	if err != nil {
		return nil, WrapError("Request", err)
	}
	return &Msg{Msg: reply}, nil
}

// RequestJSON marshals req to JSON, sends a request, and unmarshals the reply
// into resp. The context controls the deadline.
func (c *Client) RequestJSON(ctx context.Context, subject string, req any, resp any) error {
	data, err := json.Marshal(req)
	if err != nil {
		return WrapError("RequestJSON", ErrEncoding)
	}

	reply, err := c.Request(ctx, subject, data)
	if err != nil {
		return err // already wrapped
	}

	if err := json.Unmarshal(reply.Data, resp); err != nil {
		return WrapError("RequestJSON", ErrDecoding)
	}
	return nil
}
