package chat

import (
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	eventlog "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
	eventstream "github.com/JieWaZi/NeuxAlly/server/internal/chat/eventstream"
	"github.com/JieWaZi/NeuxAlly/server/internal/chat/messageview"
)

func TestSubscriptionHubReplaysLatestLiveSnapshotToNewSubscriber(t *testing.T) {
	hub := newSubscriptionHub()
	hub.publishLive("event-1", eventstream.PublicEvent{
		Transient: true,
		Message: &messageview.Message{
			ID:       "assistant-1",
			Revision: 2,
		},
	})

	_, listener := hub.add("event-1")
	select {
	case event := <-listener:
		if event.Live == nil || event.Live.Message == nil {
			t.Fatalf("replayed event = %+v, want live message", event)
		}
		if event.Live.Message.ID != "assistant-1" || event.Live.Message.Revision != 2 {
			t.Fatalf("replayed message = %+v", event.Live.Message)
		}
	default:
		t.Fatal("new subscriber did not receive latest live snapshot")
	}
}

func TestSubscriptionHubClearsLatestLiveSnapshotAtExecutionTerminal(t *testing.T) {
	hub := newSubscriptionHub()
	hub.publishLive("event-1", eventstream.PublicEvent{
		Transient: true,
		Message: &messageview.Message{
			ID:       "assistant-1",
			Revision: 2,
		},
	})
	hub.publishCommitted("event-1", []eventlog.Record{
		{Type: constants.ChatEventTypeExecutionCompleted},
	})

	_, listener := hub.add("event-1")
	select {
	case event := <-listener:
		t.Fatalf("replayed event after terminal = %+v", event)
	default:
	}
}
