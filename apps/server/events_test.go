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

func TestPublishViewerUpdatePublishesEmptySampledChannel(t *testing.T) {
	hub := newEventHub()
	events := hub.subscribe()
	defer hub.unsubscribe(events)

	published := publishViewerUpdate(
		viewerUpdate{
			SampledChannelIDs: map[string]bool{"active": true},
		},
		map[string]bool{"active": true, "other": true},
		hub,
	)
	if !published {
		t.Fatal("viewer update was not published")
	}

	event := <-events
	if event.Name != "viewers" {
		t.Fatalf("event.Name = %q, want viewers", event.Name)
	}
	var payload viewerSnapshotPayload
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if payload.Total != 0 {
		t.Fatalf("Total = %d, want 0", payload.Total)
	}
	if payload.SampledChannels != 1 {
		t.Fatalf("SampledChannels = %d, want 1", payload.SampledChannels)
	}
	if payload.TotalChannels != 2 {
		t.Fatalf("TotalChannels = %d, want 2", payload.TotalChannels)
	}
	if len(payload.Channels) != 0 {
		t.Fatalf("len(Channels) = %d, want 0", len(payload.Channels))
	}
	if len(payload.Recent) != 0 {
		t.Fatalf("len(Recent) = %d, want 0", len(payload.Recent))
	}
}
