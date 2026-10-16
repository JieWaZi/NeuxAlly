package parentwatch

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestWaitStopsWhenCallerCancels(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if Wait(ctx, os.Getpid(), time.Millisecond) {
		t.Fatal("Wait() reported current process as exited")
	}
}

func TestWaitDetectsMissingParent(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if !Wait(ctx, 2_147_483_647, time.Millisecond) {
		t.Fatal("Wait() did not report missing process")
	}
}
