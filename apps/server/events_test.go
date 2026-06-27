package main

import "testing"

func TestPublishTriggerReturnsAppliedMessageForViewerWeighting(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{{ID: "active", Name: "active"}})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	poller := newViewerPoller([]traqChannel{{ID: "active", Name: "active"}}, 1)

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

	poller.noteMessage(applied.Ch)

	poller.mu.Lock()
	weight := poller.messageWeight["active"]
	poller.mu.Unlock()
	if weight == 0 {
		t.Fatal("message weight was not updated")
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
