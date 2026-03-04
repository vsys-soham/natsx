package middleware_test

import (
	"testing"

	natslib "github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/middleware"
)

func TestRecoveryMiddleware(t *testing.T) {
	log := natsx.NopLogger{}

	panicked := false
	handler := func(m *natsx.Msg) {
		panicked = true
		panic("boom")
	}

	recovered := natsx.Chain(handler, middleware.Recovery(log))

	testMsg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.recovery"}}

	// Should NOT panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatal("panic escaped Recovery middleware")
			}
		}()
		recovered(testMsg)
	}()

	if !panicked {
		t.Fatal("handler should have panicked")
	}
}
