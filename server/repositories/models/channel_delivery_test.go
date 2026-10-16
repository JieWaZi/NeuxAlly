package models

import (
	"encoding/json"
	"testing"
)

func TestChannelEventPayloadPreservesRawJSONObject(t *testing.T) {
	event := ChannelEvent{Payload: []byte(`{"cancelingAt":42}`)}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal channel event: %v", err)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("decode channel event envelope: %v", err)
	}
	if got := string(envelope["payload"]); got != `{"cancelingAt":42}` {
		t.Fatalf("payload = %s, want raw JSON object", got)
	}

	var decoded ChannelEvent
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal channel event: %v", err)
	}
	if got := string(decoded.Payload); got != `{"cancelingAt":42}` {
		t.Fatalf("decoded payload = %s, want original JSON object", got)
	}
}
