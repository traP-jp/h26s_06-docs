package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

func TestEnsureLiveChannelDataKeepsDemoAndLiveStateSeparate(t *testing.T) {
	hits := 0
	srv, err := newServer(config{traqBaseURL: "https://example.test"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	demoState := srv.demoState
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v3/channels" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		hits++
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"public":[{"id":"root","name":"root"}]}`)),
			Request:    r,
		}, nil
	})}

	firstData, err := srv.ensureLiveChannelData(context.Background(), "token")
	if err != nil {
		t.Fatalf("first ensureLiveChannelData returned error: %v", err)
	}
	firstLiveState := firstData.State
	if firstLiveState == demoState {
		t.Fatal("live state manager shares the demo state manager")
	}
	if srv.demoState != demoState {
		t.Fatal("demo state manager was replaced by live initialization")
	}
	if _, ok := firstLiveState.applyTrigger(triggerPayload{Type: "msg", Ch: "root"}); !ok {
		t.Fatal("message trigger was not applied")
	}

	secondData, err := srv.ensureLiveChannelData(context.Background(), "token")
	if err != nil {
		t.Fatalf("second ensureLiveChannelData returned error: %v", err)
	}
	if hits != 1 {
		t.Fatalf("channels endpoint hits = %d, want 1", hits)
	}
	if secondData.State != firstLiveState {
		t.Fatal("live state manager was replaced by the second live initialization")
	}

	firstLiveState.mu.RLock()
	score := firstLiveState.channels["root"].Score
	firstLiveState.mu.RUnlock()
	if score == 0 {
		t.Fatal("score was reset after second live initialization")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
