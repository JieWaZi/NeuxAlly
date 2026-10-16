package runtimes

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingSink 线程安全地保存测试期间收到的公开 Runtime 事件。
type recordingSink struct {
	// mutex 保护多个事件交付 goroutine 对测试记录的访问。
	mutex sync.Mutex
	// events 保存 SDK 通过公开 Event Sink seam 交付的事件。
	events []Event
	// acceptedEvents 为需要精确等待交付边界的时序测试提供通知。
	acceptedEvents chan EventType
}

// newSignalingRecordingSink 创建可在事件真正被 Sink 接受后同步测试的记录器。
func newSignalingRecordingSink() *recordingSink {
	return &recordingSink{acceptedEvents: make(chan EventType, 16)}
}

// textOutput 合并当前记录中面向用户的正文增量。
func (sink *recordingSink) textOutput() string {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	var output strings.Builder
	for _, event := range sink.events {
		if event.Type != EventTypeTextDelta {
			continue
		}
		var payload struct {
			// Text 是 Runtime 返回的一段正文增量。
			Text string `json:"text"`
		}
		if json.Unmarshal(event.Data, &payload) == nil {
			output.WriteString(payload.Text)
		}
	}
	return output.String()
}

// Accept 深拷贝一条 Runtime 事件。
func (sink *recordingSink) Accept(_ context.Context, event Event) error {
	sink.mutex.Lock()
	sink.events = append(sink.events, Event{
		Type: event.Type,
		Data: append([]byte(nil), event.Data...),
	})
	sink.mutex.Unlock()
	if sink.acceptedEvents != nil {
		sink.acceptedEvents <- event.Type
	}
	return nil
}

// waitForEvent 等待目标事件越过 Event Sink 交付边界。
func (sink *recordingSink) waitForEvent(t *testing.T, expected EventType) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case eventType := <-sink.acceptedEvents:
			if eventType == expected {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for event %s; received = %#v", expected, sink.eventTypes())
		}
	}
}

// eventTypes 返回当前记录的有序事件类型快照。
func (sink *recordingSink) eventTypes() []EventType {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	types := make([]EventType, 0, len(sink.events))
	for _, event := range sink.events {
		types = append(types, event.Type)
	}
	return types
}

// containsEventType 返回事件序列是否包含目标类型。
func containsEventType(events []EventType, expected EventType) bool {
	for _, event := range events {
		if event == expected {
			return true
		}
	}
	return false
}
