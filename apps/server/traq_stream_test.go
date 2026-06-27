package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestFetchMessageInfoFetchesUserBot(t *testing.T) {
	srv, err := newServer(config{traqBaseURL: "https://example.test"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	var paths []string
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/v3/messages/message-1":
			return jsonResponse(r, `{"id":"message-1","userId":"user-1","channelId":"channel-1"}`), nil
		case "/api/v3/messages/message-2":
			return jsonResponse(r, `{"id":"message-2","userId":"user-1","channelId":"channel-2"}`), nil
		case "/api/v3/users/user-1":
			return jsonResponse(r, `{"id":"user-1","bot":true}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
			return nil, nil
		}
	})}

	channelID, isBot, err := srv.fetchMessageInfo(context.Background(), "token", "message-1")
	if err != nil {
		t.Fatalf("fetchMessageInfo returned error: %v", err)
	}
	if channelID != "channel-1" {
		t.Fatalf("channelID = %q, want %q", channelID, "channel-1")
	}
	if !isBot {
		t.Fatal("isBot = false, want true")
	}

	channelID, isBot, err = srv.fetchMessageInfo(context.Background(), "token", "message-2")
	if err != nil {
		t.Fatalf("second fetchMessageInfo returned error: %v", err)
	}
	if channelID != "channel-2" {
		t.Fatalf("second channelID = %q, want %q", channelID, "channel-2")
	}
	if !isBot {
		t.Fatal("second isBot = false, want true")
	}

	wantPaths := []string{"/api/v3/messages/message-1", "/api/v3/users/user-1", "/api/v3/messages/message-2"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("paths = %v, want %v", paths, wantPaths)
	}
}

func TestUserBotCacheReplacesRandomUserAtLimit(t *testing.T) {
	srv := &server{}

	for i := 0; i < userBotCacheLimit; i++ {
		srv.storeUserIsBot(fmt.Sprintf("user-%d", i), i%2 == 0)
	}
	srv.storeUserIsBot("overflow-user", true)

	if len(srv.userBotCache) != userBotCacheLimit {
		t.Fatalf("user bot cache size = %d, want %d", len(srv.userBotCache), userBotCacheLimit)
	}
	if isBot, ok := srv.cachedUserIsBot("overflow-user"); !ok || !isBot {
		t.Fatalf("overflow user cache = (%t, %t), want (true, true)", isBot, ok)
	}
}

func jsonResponse(r *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}
}

func TestParseTraqEventViewStateNoneReturnsClearCurrentTrigger(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	payload := mustMarshalEvent(t, wsEvent{
		Type: "USER_VIEWSTATE_CHANGED",
		Body: mustMarshalRaw(t, wsViewStateChangedBody{ViewStates: []wsViewState{
			{Key: "viewer-key", ChannelID: "channel-a", State: "none"},
		}}),
	})

	triggers, err := srv.parseTraqEvent(context.Background(), "token", payload)
	if err != nil {
		t.Fatalf("parseTraqEvent returned error: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("triggers = %d, want 1", len(triggers))
	}
	trigger := triggers[0]
	if trigger.Type != "mov" {
		t.Fatalf("Type = %q, want mov", trigger.Type)
	}
	if trigger.Usr == "" {
		t.Fatal("Usr was empty")
	}
	if trigger.From != "channel-a" {
		t.Fatalf("From = %q, want channel-a", trigger.From)
	}
	if trigger.To != "" {
		t.Fatalf("To = %q, want empty", trigger.To)
	}
	if !trigger.ClearCurrent {
		t.Fatal("ClearCurrent was false")
	}
}

func TestParseTraqEventViewStateActiveReturnsMovementTrigger(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	payload := mustMarshalEvent(t, wsEvent{
		Type: "USER_VIEWSTATE_CHANGED",
		Body: mustMarshalRaw(t, wsViewStateChangedBody{ViewStates: []wsViewState{
			{Key: "viewer-key", ChannelID: "channel-b", State: "monitoring"},
		}}),
	})

	triggers, err := srv.parseTraqEvent(context.Background(), "token", payload)
	if err != nil {
		t.Fatalf("parseTraqEvent returned error: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("triggers = %d, want 1", len(triggers))
	}
	trigger := triggers[0]
	if trigger.Type != "mov" {
		t.Fatalf("Type = %q, want mov", trigger.Type)
	}
	if trigger.From != "" {
		t.Fatalf("From = %q, want empty", trigger.From)
	}
	if trigger.To != "channel-b" {
		t.Fatalf("To = %q, want channel-b", trigger.To)
	}
	if trigger.ClearCurrent {
		t.Fatal("ClearCurrent was true")
	}
}

func mustMarshalRaw(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return data
}

func mustMarshalEvent(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return data
}

func TestParseTraqEventMessageUsesBotTokenForMessageAPI(t *testing.T) {
	var gotAuth string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/messages/message-a":
			gotAuth = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode(traqMessage{ChannelID: "channel-a", UserID: "user-a"})
		case "/api/v3/users/user-a":
			_ = json.NewEncoder(w).Encode(traqUser{Bot: false})
		default:
			t.Fatalf("path = %q, unexpected", r.URL.Path)
		}
	}))
	defer api.Close()

	srv, err := newServer(config{traqBaseURL: api.URL, traqBotAccessToken: "bot-token"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	payload := mustMarshalEvent(t, wsEvent{
		Type: "MESSAGE_CREATED",
		Body: mustMarshalRaw(t, wsMessageCreatedBody{ID: "message-a"}),
	})

	triggers, err := srv.parseTraqEvent(context.Background(), "user-token", payload)
	if err != nil {
		t.Fatalf("parseTraqEvent returned error: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("triggers = %d, want 1", len(triggers))
	}
	if gotAuth != "Bearer bot-token" {
		t.Fatalf("Authorization = %q, want Bearer bot-token", gotAuth)
	}
}
