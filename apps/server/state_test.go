package main

import (
	"encoding/json"
	"testing"
)

func TestNewStateManagerFromTraqBuildsGrandRootTree(t *testing.T) {
	parentID := "root"
	state, err := newStateManagerFromTraq([]traqChannel{
		{ID: parentID, Name: "root"},
		{ID: "child", Name: "child", ParentID: &parentID},
	})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	var payload initPayload
	if err := json.Unmarshal(state.initPayloadBytes(), &payload); err != nil {
		t.Fatalf("init payload is invalid JSON: %v", err)
	}
	if got := payload.Channels["child"].ParentID; got != parentID {
		t.Fatalf("child parent = %q, want %q", got, parentID)
	}
	if got := payload.Channels[parentID].IslandID; got != 0 {
		t.Fatalf("root island = %d, want 0", got)
	}
}

func TestStateManagerApplyTriggerSkipsDuplicateMovement(t *testing.T) {
	state, err := newDemoStateManager()
	if err != nil {
		t.Fatalf("newDemoStateManager returned error: %v", err)
	}
	channelID := state.randomLeafID()

	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", To: channelID}); !ok {
		t.Fatal("first movement was not applied")
	}
	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", To: channelID}); ok {
		t.Fatal("duplicate movement was applied")
	}
}
