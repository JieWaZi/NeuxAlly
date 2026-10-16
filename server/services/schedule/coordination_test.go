package schedule

import (
	"context"
	"testing"
	"time"
)

func TestMutationScopeNestedReleaseKeepsOuterProtection(t *testing.T) {
	service := New(Config{})
	ctx, release := service.AcquireMutation(context.Background())
	_, nestedRelease := service.AcquireMutation(ctx)
	nestedRelease()
	nestedRelease()
	entered := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_, finish := service.AcquireMutation(context.Background())
		defer finish()
		close(entered)
		close(done)
	}()
	select {
	case <-entered:
		t.Fatal("nested release unlocked outer mutation")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("outer release did not release mutation")
	}
}

func TestReleasedMutationContextMustAcquireAgain(t *testing.T) {
	service := New(Config{})
	expired, release := service.AcquireMutation(context.Background())
	release()
	_, activeRelease := service.AcquireMutation(context.Background())
	entered := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_, finish := service.AcquireMutation(expired)
		defer finish()
		close(entered)
		close(done)
	}()
	select {
	case <-entered:
		t.Fatal("released context bypassed an active mutation")
	case <-time.After(20 * time.Millisecond):
	}
	activeRelease()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expired context could not acquire mutation again")
	}
}
