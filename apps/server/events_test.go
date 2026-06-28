package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishTriggerReturnsAppliedMessageForViewerSampling(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{{ID: "active", Name: "active"}})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	applied, published := srv.publishTrigger(
		triggerPayload{Type: "msg", Ch: "active"},
		map[string]bool{"active": true},
		state,
		newEventHub(),
	)
	if !published {
		t.Fatal("message trigger was not published")
	}
	if applied.Type != "msg" || applied.Ch != "active" {
		t.Fatalf("applied trigger = %#v, want msg active", applied)
	}

	state.mu.RLock()
	score := state.channels["active"].Score
	state.mu.RUnlock()
	if score == 0 {
		t.Fatal("message score was not updated for viewer sampling")
	}
}

func TestTriggerInActiveChannelsAllowsClearCurrentFromActiveChannel(t *testing.T) {
	active := map[string]bool{"from": true}

	if !triggerInActiveChannels(triggerPayload{Type: "mov", From: "from", ClearCurrent: true}, active) {
		t.Fatal("clear current trigger from active channel was rejected")
	}
	if triggerInActiveChannels(triggerPayload{Type: "mov", From: "inactive", ClearCurrent: true}, active) {
		t.Fatal("clear current trigger from inactive channel was accepted")
	}
}

func TestTriggerPayloadAlwaysIncludesDelta(t *testing.T) {
	data, err := json.Marshal(triggerPayload{Type: "msg", Ch: "active"})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if !strings.Contains(string(data), `"delta":0`) {
		t.Fatalf("payload = %s, want zero delta field", data)
	}
}
