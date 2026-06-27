package main

import (
	"context"
	"net/http"
	"testing"
	"time"
)

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
