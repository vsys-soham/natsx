package envelope_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vsys-soham/natsx/envelope"
)

type Order struct {
	ID   string `json:"id"`
	Item string `json:"item"`
	Qty  int    `json:"qty"`
}

func TestWrap(t *testing.T) {
	o := Order{ID: "001", Item: "Widget", Qty: 5}
	env := envelope.Wrap(o)

	if env.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if env.Timestamp.IsZero() {
		t.Fatal("expected non-zero Timestamp")
	}
	if env.Data.ID != "001" {
		t.Fatalf("expected Data.ID=001, got %q", env.Data.ID)
	}
}

func TestWrapWith(t *testing.T) {
	o := Order{ID: "002", Item: "Gadget", Qty: 3}
	env := envelope.WrapWith(o, "order.created", "order-svc", "corr-xyz")

	if env.Type != "order.created" {
		t.Fatalf("Type=%q, want order.created", env.Type)
	}
	if env.Source != "order-svc" {
		t.Fatalf("Source=%q, want order-svc", env.Source)
	}
	if env.CorrelationID != "corr-xyz" {
		t.Fatalf("CorrelationID=%q, want corr-xyz", env.CorrelationID)
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	o := Order{ID: "003", Item: "Doohickey", Qty: 10}
	env := envelope.WrapWith(o, "order.created", "svc", "c-1")

	data, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	decoded, err := envelope.Unmarshal[Order](data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != env.ID {
		t.Fatalf("ID mismatch: %q vs %q", decoded.ID, env.ID)
	}
	if decoded.Type != "order.created" {
		t.Fatalf("Type=%q, want order.created", decoded.Type)
	}
	if decoded.Data.Item != "Doohickey" {
		t.Fatalf("Data.Item=%q, want Doohickey", decoded.Data.Item)
	}
	if decoded.Data.Qty != 10 {
		t.Fatalf("Data.Qty=%d, want 10", decoded.Data.Qty)
	}
}

func TestUnmarshalInvalid(t *testing.T) {
	_, err := envelope.Unmarshal[Order]([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUniqueIDs(t *testing.T) {
	ids := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		env := envelope.Wrap(i)
		if _, exists := ids[env.ID]; exists {
			t.Fatalf("duplicate ID: %s", env.ID)
		}
		ids[env.ID] = struct{}{}
	}
}

func TestTimestampIsUTC(t *testing.T) {
	env := envelope.Wrap("hello")
	if env.Timestamp.Location() != time.UTC {
		t.Fatalf("expected UTC, got %v", env.Timestamp.Location())
	}
}

func TestJSONFieldNames(t *testing.T) {
	env := envelope.WrapWith("test", "t", "s", "c")
	data, _ := env.Marshal()

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	required := []string{"id", "type", "source", "correlation_id", "timestamp", "data"}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("missing JSON field %q", f)
		}
	}
}
