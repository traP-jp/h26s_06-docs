package main

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestStateManagerApplyTriggerSkipsDuplicateMessage(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{{ID: "root", Name: "root"}})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	trigger := triggerPayload{Type: "msg", Ch: "root", MessageID: "message-1"}
	if _, ok := state.applyTrigger(trigger); !ok {
		t.Fatal("first message was not applied")
	}
	if _, ok := state.applyTrigger(trigger); ok {
		t.Fatal("duplicate message was applied")
	}

	state.mu.RLock()
	score := state.channels["root"].Score
	state.mu.RUnlock()
	if score != 46 {
		t.Fatalf("root score = %v, want 46", score)
	}
}

func TestStateManagerKeepsOnlyRecentMessageIDs(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{{ID: "root", Name: "root"}})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	for i := 0; i < recentMessageIDLimit+1; i++ {
		trigger := triggerPayload{Type: "msg", Ch: "root", MessageID: fmt.Sprintf("message-%d", i)}
		if _, ok := state.applyTrigger(trigger); !ok {
			t.Fatalf("message %d was not applied", i)
		}
	}

	state.mu.RLock()
	seenCount := len(state.seenMessageIDs)
	recentCount := len(state.recentMessageIDs)
	_, firstStillSeen := state.seenMessageIDs["message-0"]
	state.mu.RUnlock()
	if seenCount != recentMessageIDLimit {
		t.Fatalf("seen message IDs = %d, want %d", seenCount, recentMessageIDLimit)
	}
	if recentCount != recentMessageIDLimit {
		t.Fatalf("recent message IDs = %d, want %d", recentCount, recentMessageIDLimit)
	}
	if firstStillSeen {
		t.Fatal("oldest message ID was not evicted")
	}
	if _, ok := state.applyTrigger(triggerPayload{Type: "msg", Ch: "root", MessageID: "message-0"}); !ok {
		t.Fatal("evicted message ID was still treated as duplicate")
	}
}

func TestViewerPollWeightUsesCurrentScoreAndElapsed(t *testing.T) {
	if got := viewerPollWeight(0, 0); got != 0 {
		t.Fatalf("weight = %v, want 0", got)
	}
	if got := viewerPollWeight(50, 10); got < 0.509 || got > 0.511 {
		t.Fatalf("weight = %v, want 0.51", got)
	}
	if got := viewerPollWeight(100, 100); got != 1.1 {
		t.Fatalf("weight = %v, want 1.1", got)
	}
}

func TestNormalizeWeightedChannelsSumsToOne(t *testing.T) {
	normalized := normalizeWeightedChannels([]weightedChannel{
		{id: "a", rawWeight: 2},
		{id: "b", rawWeight: 3},
		{id: "c", rawWeight: 0},
	})
	if len(normalized) != 2 {
		t.Fatalf("normalized channels = %d, want 2", len(normalized))
	}
	total := 0.0
	for _, channel := range normalized {
		total += channel.normalizedWeight
	}
	if total < 0.999 || total > 1.001 {
		t.Fatalf("normalized total = %v, want 1", total)
	}
}

func TestStateManagerSampleViewerChannelsCapsInitialSelection(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{
		{ID: "a", Name: "a"},
		{ID: "b", Name: "b"},
		{ID: "c", Name: "c"},
	})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}
	candidates := []traqChannel{
		{ID: "a", Name: "a"},
		{ID: "b", Name: "b"},
		{ID: "c", Name: "c"},
	}

	selected := state.sampleViewerChannels(candidates, 2)
	if len(selected) != 2 {
		t.Fatalf("selected channels = %d, want 2", len(selected))
	}
	for _, selectedChannel := range selected {
		state.mu.RLock()
		lastViewTime := state.channels[selectedChannel.ID].LastViewTime
		state.mu.RUnlock()
		if lastViewTime.IsZero() {
			t.Fatalf("selected channel %s did not record last view time", selectedChannel.ID)
		}
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
