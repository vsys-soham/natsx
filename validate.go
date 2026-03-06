package natsx

import (
	"fmt"
	"strings"
)

// ValidateSubject checks that a NATS subject is well-formed.
// It returns an error if the subject is empty, contains spaces,
// or starts/ends with a dot.
func ValidateSubject(subject string) error {
	if subject == "" {
		return fmt.Errorf("%w: subject is empty", ErrInvalidSubj)
	}
	if strings.ContainsAny(subject, " \t\r\n") {
		return fmt.Errorf("%w: subject contains whitespace: %q", ErrInvalidSubj, subject)
	}
	if strings.HasPrefix(subject, ".") || strings.HasSuffix(subject, ".") {
		return fmt.Errorf("%w: subject starts or ends with dot: %q", ErrInvalidSubj, subject)
	}
	if strings.Contains(subject, "..") {
		return fmt.Errorf("%w: subject contains empty token: %q", ErrInvalidSubj, subject)
	}
	return nil
}
