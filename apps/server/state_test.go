package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestStateManagerApplyTriggerClearsCurrentChannelWithoutPublishing(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{
		{ID: "a", Name: "a"},
		{ID: "b", Name: "b"},
	})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", To: "a"}); !ok {
		t.Fatal("initial movement was not applied")
	}
	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", From: "a", ClearCurrent: true}); ok {
		t.Fatal("clear current trigger was published")
	}
	state.mu.RLock()
	current := state.users["u1"].CurrentChannel
	state.mu.RUnlock()
	if current != "" {
		t.Fatalf("CurrentChannel = %q, want empty", current)
	}

	applied, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", To: "b"})
	if !ok {
		t.Fatal("movement after clear was not applied")
	}
	if applied.From != "" {
		t.Fatalf("inferred From = %q, want empty", applied.From)
	}
}

func TestStateManagerApplyTriggerIgnoresStaleClearCurrent(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{
		{ID: "a", Name: "a"},
		{ID: "b", Name: "b"},
	})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", To: "a"}); !ok {
		t.Fatal("first movement was not applied")
	}
	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", To: "b"}); !ok {
		t.Fatal("second movement was not applied")
	}
	if _, ok := state.applyTrigger(triggerPayload{Type: "mov", Usr: "u1", From: "a", ClearCurrent: true}); ok {
		t.Fatal("stale clear current trigger was published")
	}

	state.mu.RLock()
	current := state.users["u1"].CurrentChannel
	state.mu.RUnlock()
	if current != "b" {
		t.Fatalf("CurrentChannel = %q, want b", current)
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

func TestStateManagerSyncPayloadCapsDeltasAtOneHundred(t *testing.T) {
	channels := make([]traqChannel, 0, maxSyncPayloadDeltas+1)
	for i := 0; i < maxSyncPayloadDeltas+1; i++ {
		id := fmt.Sprintf("ch-%d", i)
		channels = append(channels, traqChannel{ID: id, Name: id})
	}
	state, err := newStateManagerFromTraq(channels)
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}
	state.mu.Lock()
	now := time.Now()
	for id, ch := range state.channels {
		if id == grandRootID {
			continue
		}
		ch.Score = 10
		ch.LastSyncScore = 0
		ch.LastSyncTime = now.Add(-time.Minute)
		ch.LastDecayTime = now
	}
	state.mu.Unlock()

	payload := state.syncPayload()
	if len(payload.Deltas) != maxSyncPayloadDeltas {
		t.Fatalf("sync deltas = %d, want %d", len(payload.Deltas), maxSyncPayloadDeltas)
	}
}

func TestStateManagerSyncPayloadDoesNotDoubleDecayUnselectedChannels(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{
		{ID: "selected", Name: "selected"},
		{ID: "unselected", Name: "unselected"},
	})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	oldDecay := time.Now().Add(-24 * time.Second)
	unselectedSync := time.Now().Add(20 * time.Minute)
	state.mu.Lock()
	state.channels["selected"].Score = 10
	state.channels["selected"].LastSyncScore = 0
	state.channels["selected"].LastSyncTime = time.Now().Add(-time.Minute)
	state.channels["selected"].LastDecayTime = oldDecay
	state.channels["unselected"].Score = 10
	state.channels["unselected"].LastSyncScore = 10
	state.channels["unselected"].LastSyncTime = unselectedSync
	state.channels["unselected"].LastDecayTime = oldDecay
	state.mu.Unlock()

	payload := state.syncPayload()
	if _, ok := payload.Deltas["selected"]; !ok {
		t.Fatal("selected channel was not synced")
	}

	state.mu.RLock()
	unselected := state.channels["unselected"]
	score := unselected.Score
	lastSync := unselected.LastSyncTime
	lastDecay := unselected.LastDecayTime
	state.mu.RUnlock()
	if !lastDecay.After(oldDecay) {
		t.Fatal("unselected channel decay time was not updated")
	}
	if !lastSync.Equal(unselectedSync) {
		t.Fatal("unselected channel sync time was updated")
	}
	want := 10 * math.Exp(-24.0/24.0)
	if math.Abs(score-want) > 0.1 {
		t.Fatalf("unselected score = %v, want about %v", score, want)
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
