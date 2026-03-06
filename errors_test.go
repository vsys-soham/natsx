package natsx_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil", nil, false},
		{"encoding", natsx.ErrEncoding, false},
		{"decoding", natsx.ErrDecoding, false},
		{"invalid subject", natsx.ErrInvalidSubj, false},
		{"bad payload", natsx.ErrBadPayload, false},
		{"nats timeout", nats.ErrTimeout, true},
		{"nats no responders", nats.ErrNoResponders, true},
		{"nats conn closed", nats.ErrConnectionClosed, true},
		{"nats disconnected", nats.ErrDisconnected, true},
		{"nats slow consumer", nats.ErrSlowConsumer, true},
		{"wrapped encoding", natsx.WrapError("Publish", natsx.ErrEncoding), false},
		{"wrapped timeout", natsx.WrapError("Request", nats.ErrTimeout), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := natsx.IsRetryable(tt.err)
			if got != tt.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}
