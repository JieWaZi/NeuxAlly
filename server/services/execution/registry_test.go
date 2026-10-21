package execution

import (
	"context"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
)

func TestExecutionRegistryCloseWaitsForReservedExecution(t *testing.T) {
	registry := newExecutionRegistry()
	registry.start(context.Background())
	executionContext, err := registry.reserve("chat-1", "execution-1")
	if err != nil {
		t.Fatalf("reserve() error = %v", err)
	}

	closed := make(chan struct{})
	go func() {
		registry.close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("close() returned before the reserved execution was released")
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-executionContext.Done():
	case <-time.After(time.Second):
		t.Fatal("reserved execution context was not canceled by close()")
	}

	registry.release("chat-1", "execution-1")
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close() remained blocked after the reservation was released")
	}
}

func TestExecutionRegistryCloseWaitsForLaunchedExecutionWithoutDoubleRelease(
	t *testing.T,
) {
	registry := newExecutionRegistry()
	registry.start(context.Background())
	executionContext, err := registry.reserve("chat-1", "execution-1")
	if err != nil {
		t.Fatalf("reserve() error = %v", err)
	}
	runStarted := make(chan struct{})
	runFinished := make(chan struct{})
	registry.launch(
		"chat-1",
		"execution-1",
		executionContext,
		func(ctx context.Context) {
			close(runStarted)
			<-ctx.Done()
			close(runFinished)
		},
	)
	<-runStarted

	closed := make(chan struct{})
	go func() {
		registry.close()
		close(closed)
	}()
	select {
	case <-runFinished:
	case <-time.After(time.Second):
		t.Fatal("launched execution did not observe close cancellation")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close() did not wait for launched execution cleanup")
	}

	// 后台 defer 再次 release 时必须保持幂等，不能重复 Done 或关闭通道。
	registry.release("chat-1", "execution-1")
}

func TestExecutionRegistryCancelAndWaitCoversCompletionAndTimeout(t *testing.T) {
	registry := newExecutionRegistry()
	registry.close()
	registry.start(context.Background())
	registry.start(context.Background())
	if err := registry.cancelAndWait(context.Background(), "missing"); err != nil {
		t.Fatalf("cancelAndWait(missing) error = %v", err)
	}

	if _, err := registry.reserve("chat-complete", "execution-complete"); err != nil {
		t.Fatalf("reserve(completion) error = %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		registry.release("chat-complete", "execution-complete")
	}()
	if err := registry.cancelAndWait(context.Background(), "chat-complete"); err != nil {
		t.Fatalf("cancelAndWait(completion) error = %v", err)
	}

	if _, err := registry.reserve("chat-timeout", "execution-timeout"); err != nil {
		t.Fatalf("reserve(timeout) error = %v", err)
	}
	timeoutContext, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := registry.cancelAndWait(timeoutContext, "chat-timeout"); err !=
		status_error.ChatHasActiveExecution {
		t.Fatalf("cancelAndWait(timeout) error = %v", err)
	}
	registry.release("chat-timeout", "execution-timeout")
	registry.close()
}

// 捕获旧执行后新回合占用同一聊天时，停止必须只作用于原身份。
func TestCancelExecutionDoesNotStopNextTurn(t *testing.T) {
	registry := newExecutionRegistry()
	registry.start(context.Background())
	_, err := registry.reserve("chat", "old")
	if err != nil {
		t.Fatal(err)
	}
	registry.release("chat", "old")
	next, err := registry.reserve("chat", "next")
	if err != nil {
		t.Fatal(err)
	}
	defer registry.release("chat", "next")
	manager := &service{executions: registry}
	canceller, ok := any(manager).(interface{ CancelExecution(string, string) error })
	if !ok {
		t.Fatal("缺少按已捕获执行身份取消入口")
	}
	if err := canceller.CancelExecution("chat", "old"); err != nil {
		t.Fatal(err)
	}
	if next.Err() != nil {
		t.Fatal("旧回合停止误伤新执行")
	}
	if err := canceller.CancelExecution("chat", "next"); err != nil {
		t.Fatal(err)
	}
	if next.Err() == nil {
		t.Fatal("指定回合未被停止")
	}
}
