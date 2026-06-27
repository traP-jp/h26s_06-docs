package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActiveTraqChannelsSkipsArchivedAncestors(t *testing.T) {
	archivedParent := "archived-parent"
	active := activeTraqChannels([]traqChannel{
		{ID: "active", Name: "active"},
		{ID: archivedParent, Name: "archived", Archived: true},
		{ID: "child", Name: "child", ParentID: &archivedParent},
		{Name: "empty"},
	})

	if len(active) != 1 {
		t.Fatalf("len(active) = %d, want 1", len(active))
	}
	if active[0].ID != "active" {
		t.Fatalf("active[0].ID = %q, want active", active[0].ID)
	}
}

func TestTriggerInActiveChannels(t *testing.T) {
	active := map[string]bool{"active": true}
	tests := []struct {
		name    string
		trigger triggerPayload
		want    bool
	}{
		{name: "message active", trigger: triggerPayload{Type: "msg", Ch: "active"}, want: true},
		{name: "message inactive", trigger: triggerPayload{Type: "msg", Ch: "inactive"}, want: false},
		{name: "movement active", trigger: triggerPayload{Type: "mov", To: "active"}, want: true},
		{name: "movement inactive", trigger: triggerPayload{Type: "mov", To: "inactive"}, want: false},
		{name: "unknown", trigger: triggerPayload{Type: "other", Ch: "active"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := triggerInActiveChannels(tt.trigger, active); got != tt.want {
				t.Fatalf("triggerInActiveChannels() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestEnsureLiveChannelDataUsesBotTokenWhenConfigured(t *testing.T) {
	var gotAuth string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/channels" {
			t.Fatalf("path = %q, want /api/v3/channels", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(traqChannelList{Public: []traqChannel{{ID: "root", Name: "root"}}})
	}))
	defer api.Close()

	srv, err := newServer(config{traqBaseURL: api.URL, traqBotAccessToken: "bot-token"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	if _, err := srv.ensureLiveChannelData(context.Background(), "user-token"); err != nil {
		t.Fatalf("ensureLiveChannelData returned error: %v", err)
	}
	if gotAuth != "Bearer bot-token" {
		t.Fatalf("Authorization = %q, want Bearer bot-token", gotAuth)
	}
}

func TestEnsureLiveChannelDataRequiresBotToken(t *testing.T) {
	var gotAuth string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(traqChannelList{Public: []traqChannel{{ID: "root", Name: "root"}}})
	}))
	defer api.Close()

	srv, err := newServer(config{traqBaseURL: api.URL, traqBotAccessToken: "bot-token"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	if _, err := srv.ensureLiveChannelData(context.Background(), "user-token"); err != nil {
		t.Fatalf("ensureLiveChannelData returned error: %v", err)
	}
	if gotAuth != "Bearer bot-token" {
		t.Fatalf("Authorization = %q, want Bearer bot-token", gotAuth)
	}
}
