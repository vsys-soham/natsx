package natsx_test

import (
	"errors"
	"testing"

	"github.com/vsys-soham/natsx"
)

func TestValidateSubject(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{"valid simple", "foo", false},
		{"valid dotted", "foo.bar.baz", false},
		{"valid wildcard", "foo.*", false},
		{"valid multi-wildcard", "foo.>", false},
		{"empty", "", true},
		{"has space", "foo bar", true},
		{"has tab", "foo\tbar", true},
		{"has newline", "foo\nbar", true},
		{"starts with dot", ".foo", true},
		{"ends with dot", "foo.", true},
		{"double dot", "foo..bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := natsx.ValidateSubject(tt.subject)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.subject)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.subject, err)
			}
			if err != nil && !errors.Is(err, natsx.ErrInvalidSubj) {
				t.Errorf("error should wrap ErrInvalidSubj, got %v", err)
			}
		})
	}
}
