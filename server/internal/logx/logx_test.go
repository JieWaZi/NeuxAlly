package logx

import (
	"io"
	"sync"
	"testing"

	ginxlogx "github.com/shrewx/ginx/pkg/logx"
)

func TestConcurrentLoggingUsesSerializedBoundary(t *testing.T) {
	logger := ginxlogx.Instance()
	originalOutput := logger.Out
	logger.SetOutput(io.Discard)
	t.Cleanup(func() { logger.SetOutput(originalOutput) })

	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for message := 0; message < 16; message++ {
				Infof("concurrent log boundary worker=%d message=%d", worker, message)
			}
		}(worker)
	}
	wait.Wait()
}
