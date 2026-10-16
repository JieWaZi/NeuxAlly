package channel

import (
	"testing"
	"time"
)

func TestScheduleLocksSerializePerThreadWithoutBlockingOtherTeams(t *testing.T) {
	service := &service{}
	releaseFirst := service.acquireScheduleLock("thread-a")
	otherAcquired := make(chan struct{})
	go func() {
		release := service.acquireScheduleLock("thread-b")
		close(otherAcquired)
		release()
	}()
	select {
	case <-otherAcquired:
	case <-time.After(time.Second):
		t.Fatal("thread-b was blocked by thread-a schedule lock")
	}
	sameAcquired := make(chan struct{})
	go func() {
		release := service.acquireScheduleLock("thread-a")
		close(sameAcquired)
		release()
	}()
	select {
	case <-sameAcquired:
		t.Fatal("same Team acquired schedule lock concurrently")
	case <-time.After(50 * time.Millisecond):
	}
	releaseFirst()
	select {
	case <-sameAcquired:
	case <-time.After(time.Second):
		t.Fatal("same Team did not acquire schedule lock after release")
	}
}
