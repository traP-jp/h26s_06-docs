package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParseTraqEventViewStateNoneReturnsClearCurrentTrigger(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	payload := mustMarshalEvent(t, wsEvent{
		Type: "USER_VIEWSTATE_CHANGED",
		Body: mustMarshalRaw(t, wsViewStateChangedBody{ViewStates: []wsViewState{
			{Key: "viewer-key", ChannelID: "channel-a", State: "none"},
		}}),
	})

	triggers, err := srv.parseTraqEvent(context.Background(), "token", payload)
	if err != nil {
		t.Fatalf("parseTraqEvent returned error: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("triggers = %d, want 1", len(triggers))
	}
	trigger := triggers[0]
	if trigger.Type != "mov" {
		t.Fatalf("Type = %q, want mov", trigger.Type)
	}
	if trigger.Usr == "" {
		t.Fatal("Usr was empty")
	}
	if trigger.From != "channel-a" {
		t.Fatalf("From = %q, want channel-a", trigger.From)
	}
	if trigger.To != "" {
		t.Fatalf("To = %q, want empty", trigger.To)
	}
	if !trigger.ClearCurrent {
		t.Fatal("ClearCurrent was false")
	}
}

func TestParseTraqEventViewStateActiveReturnsMovementTrigger(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	payload := mustMarshalEvent(t, wsEvent{
		Type: "USER_VIEWSTATE_CHANGED",
		Body: mustMarshalRaw(t, wsViewStateChangedBody{ViewStates: []wsViewState{
			{Key: "viewer-key", ChannelID: "channel-b", State: "monitoring"},
		}}),
	})

	triggers, err := srv.parseTraqEvent(context.Background(), "token", payload)
	if err != nil {
		t.Fatalf("parseTraqEvent returned error: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("triggers = %d, want 1", len(triggers))
	}
	trigger := triggers[0]
	if trigger.Type != "mov" {
		t.Fatalf("Type = %q, want mov", trigger.Type)
	}
	if trigger.From != "" {
		t.Fatalf("From = %q, want empty", trigger.From)
	}
	if trigger.To != "channel-b" {
		t.Fatalf("To = %q, want channel-b", trigger.To)
	}
	if trigger.ClearCurrent {
		t.Fatal("ClearCurrent was true")
	}
}

func mustMarshalRaw(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return data
}

func mustMarshalEvent(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return data
}
