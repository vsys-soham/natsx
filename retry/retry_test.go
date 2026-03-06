package retry_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vsys-soham/natsx/retry"
)

func TestDoSuccess(t *testing.T) {
	p := retry.DefaultPolicy()
	res := retry.Do(context.Background(), p, func(_ context.Context, attempt int) error {
		return nil
	})
	if res.Err != nil {
		t.Fatalf("expected no error, got %v", res.Err)
	}
	if res.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", res.Attempts)
	}
}

func TestDoRetryThenSucceed(t *testing.T) {
	p := retry.Policy{
		MaxAttempts:    5,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
	}

	var calls atomic.Int32
	res := retry.Do(context.Background(), p, func(_ context.Context, attempt int) error {
		calls.Add(1)
		if attempt < 2 {
			return errors.New("transient")
		}
		return nil
	})

	if res.Err != nil {
		t.Fatalf("expected success, got %v", res.Err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
	if res.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", res.Attempts)
	}
}

func TestDoExhausted(t *testing.T) {
	p := retry.Policy{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Multiplier:     1.0,
	}

	res := retry.Do(context.Background(), p, func(_ context.Context, _ int) error {
		return errors.New("always fails")
	})

	if res.Err == nil {
		t.Fatal("expected error after exhaustion")
	}
	if res.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", res.Attempts)
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	p := retry.DefaultPolicy()
	res := retry.Do(ctx, p, func(_ context.Context, _ int) error {
		return errors.New("should not reach")
	})

	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", res.Err)
	}
}

func TestDoPermanentError(t *testing.T) {
	p := retry.Policy{
		MaxAttempts:    5,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Multiplier:     1.0,
		Classifier:     retry.ClassifyPermanent,
	}

	permErr := &retry.Permanent{Err: errors.New("fatal")}

	var calls atomic.Int32
	res := retry.Do(context.Background(), p, func(_ context.Context, _ int) error {
		calls.Add(1)
		return permErr
	})

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 call (permanent), got %d", got)
	}
	if res.Err == nil {
		t.Fatal("expected error")
	}
}

func TestNoRetry(t *testing.T) {
	p := retry.NoRetry()
	var calls atomic.Int32
	res := retry.Do(context.Background(), p, func(_ context.Context, _ int) error {
		calls.Add(1)
		return errors.New("fail")
	})

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 call with NoRetry, got %d", got)
	}
	if res.Err == nil {
		t.Fatal("expected error")
	}
}

func TestIsPermanent(t *testing.T) {
	inner := errors.New("bad input")
	perm := &retry.Permanent{Err: inner}

	if !retry.IsPermanent(perm) {
		t.Fatal("expected IsPermanent=true")
	}
	if retry.IsPermanent(inner) {
		t.Fatal("expected IsPermanent=false for regular error")
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := retry.DefaultPolicy()
	if p.MaxAttempts != 3 {
		t.Errorf("MaxAttempts=%d, want 3", p.MaxAttempts)
	}
	if p.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff=%v, want 100ms", p.InitialBackoff)
	}
	if p.MaxBackoff != 5*time.Second {
		t.Errorf("MaxBackoff=%v, want 5s", p.MaxBackoff)
	}
	if p.Multiplier != 2.0 {
		t.Errorf("Multiplier=%v, want 2.0", p.Multiplier)
	}
}
