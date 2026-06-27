package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFetchViewerSnapshotTotalCountsRowsBeforeRecentTruncation(t *testing.T) {
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	srv := newViewerSnapshotTestServer(t, map[string][]traqViewer{
		"general": newTestViewers(30, "monitoring", base),
	})
	poller := newViewerSnapshotTestPoller(t, []traqChannel{{ID: "general", Name: "general"}}, 1)

	snapshot, err := srv.fetchViewerSnapshot(context.Background(), "token", poller)
	if err != nil {
		t.Fatalf("fetchViewerSnapshot returned error: %v", err)
	}

	if snapshot.Total != 30 {
		t.Fatalf("Total = %d, want 30", snapshot.Total)
	}
	if len(snapshot.Recent) != 24 {
		t.Fatalf("len(Recent) = %d, want 24", len(snapshot.Recent))
	}
	if snapshot.Total <= len(snapshot.Recent) {
		t.Fatalf("Total = %d, len(Recent) = %d; want Total to exceed Recent length", snapshot.Total, len(snapshot.Recent))
	}
}

func TestFetchViewerSnapshotSummariesCountFullViewers(t *testing.T) {
	base := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	viewers := append(newTestViewers(13, "monitoring", base), newTestViewers(7, "editing", base.Add(13*time.Minute))...)
	viewers = append(viewers, newTestViewers(5, "stale_viewing", base.Add(20*time.Minute))...)
	srv := newViewerSnapshotTestServer(t, map[string][]traqViewer{
		"random": viewers,
	})
	poller := newViewerSnapshotTestPoller(t, []traqChannel{{ID: "random", Name: "random"}}, 1)

	snapshot, err := srv.fetchViewerSnapshot(context.Background(), "token", poller)
	if err != nil {
		t.Fatalf("fetchViewerSnapshot returned error: %v", err)
	}
	if len(snapshot.Channels) != 1 {
		t.Fatalf("len(Channels) = %d, want 1", len(snapshot.Channels))
	}
	if snapshot.Total != 25 {
		t.Fatalf("Total = %d, want 25", snapshot.Total)
	}

	summary := snapshot.Channels[0]
	if summary.Count != 25 {
		t.Fatalf("summary.Count = %d, want 25", summary.Count)
	}
	if summary.Monitoring != 13 {
		t.Fatalf("summary.Monitoring = %d, want 13", summary.Monitoring)
	}
	if summary.Editing != 7 {
		t.Fatalf("summary.Editing = %d, want 7", summary.Editing)
	}
	if summary.Stale != 5 {
		t.Fatalf("summary.Stale = %d, want 5", summary.Stale)
	}
	if len(snapshot.Recent) != 24 {
		t.Fatalf("len(Recent) = %d, want 24", len(snapshot.Recent))
	}
}

func TestStreamViewerSnapshotsSkipsFetchWithoutSubscribers(t *testing.T) {
	state, err := newStateManagerFromTraq([]traqChannel{{ID: "root", Name: "root"}})
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}

	requests := 0
	srv := &server{
		cfg: config{
			traqBaseURL:        "https://example.test",
			viewerPollInterval: time.Millisecond,
		},
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests++
			return nil, context.Canceled
		})},
	}
	poller := newViewerPoller([]traqChannel{{ID: "root", Name: "root"}}, 1, state)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	for range srv.streamViewerSnapshots(ctx, "token", poller, newEventHub()) {
		t.Fatal("snapshot was emitted without subscribers")
	}
	if requests != 0 {
		t.Fatalf("viewer API requests = %d, want 0", requests)
	}
}

func newViewerSnapshotTestServer(t *testing.T, viewersByChannel map[string][]traqViewer) *server {
	t.Helper()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		channelID, ok := strings.CutPrefix(r.URL.Path, "/api/v3/channels/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		channelID, ok = strings.CutSuffix(channelID, "/viewers")
		if !ok {
			http.NotFound(w, r)
			return
		}
		viewers, ok := viewersByChannel[channelID]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(viewers); err != nil {
			t.Errorf("encoding viewers: %v", err)
		}
	}))
	t.Cleanup(api.Close)

	srv, err := newServer(config{traqBaseURL: api.URL})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	t.Cleanup(srv.close)
	return srv
}

func newViewerSnapshotTestPoller(t *testing.T, channels []traqChannel, maxPerTick int) *viewerPoller {
	t.Helper()

	state, err := newStateManagerFromTraq(channels)
	if err != nil {
		t.Fatalf("newStateManagerFromTraq returned error: %v", err)
	}
	return newViewerPoller(channels, maxPerTick, state)
}

func newTestViewers(count int, state string, base time.Time) []traqViewer {
	viewers := make([]traqViewer, 0, count)
	for i := range count {
		viewers = append(viewers, traqViewer{
			UserID:    state + "-user-" + strconv.Itoa(i),
			State:     state,
			UpdatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return viewers
}
