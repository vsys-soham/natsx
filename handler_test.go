package natsx_test

import (
	"testing"

	"github.com/vsys-soham/natsx"
)

func TestChainOrdering(t *testing.T) {
	var trace []string

	mwA := func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(m *natsx.Msg) {
			trace = append(trace, "A")
			next(m)
		}
	}
	mwB := func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(m *natsx.Msg) {
			trace = append(trace, "B")
			next(m)
		}
	}
	handler := func(m *natsx.Msg) {
		trace = append(trace, "H")
	}

	chained := natsx.Chain(handler, mwA, mwB)
	chained(nil) // Msg not used in this test

	expected := []string{"A", "B", "H"}
	if len(trace) != len(expected) {
		t.Fatalf("trace length %d, want %d", len(trace), len(expected))
	}
	for i, v := range expected {
		if trace[i] != v {
			t.Fatalf("trace[%d]=%q, want %q", i, trace[i], v)
		}
	}
}
