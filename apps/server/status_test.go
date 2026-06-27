package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleStatusStoresCurrentChannel(t *testing.T) {
	srv, err := newServer(config{
		traqBaseURL:        "https://example.test",
		traqBotAccessToken: "bot-token",
	})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.sessions["session-id"] = authSession{
		Token:     tokenResponse{AccessToken: "user-token"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/channels":
			return jsonResponse(r, `{"public":[{"id":"channel-a","name":"channel-a"}]}`), nil
		case "/api/v3/users/me":
			return jsonResponse(r, `{"id":"user-a","name":"user_a"}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
			return nil, nil
		}
	})}

	req := httptest.NewRequest(http.MethodPut, "/api/status", strings.NewReader(`{"channel":"channel-a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body=%q", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	srv.liveData.State.mu.RLock()
	user := srv.liveData.State.users["user-a"]
	srv.liveData.State.mu.RUnlock()
	if user == nil {
		t.Fatal("user status was not stored")
	}
	if user.CurrentChannel != "channel-a" {
		t.Fatalf("CurrentChannel = %q, want channel-a", user.CurrentChannel)
	}
	if user.LastUpdated.IsZero() {
		t.Fatal("LastUpdated was zero")
	}
}

func TestHandleStatusRequiresAuthentication(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/status", strings.NewReader(`{"channel":"channel-a"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleStatusRejectsMissingChannel(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.sessions["session-id"] = authSession{
		Token:     tokenResponse{AccessToken: "user-token"},
		ExpiresAt: time.Now().Add(time.Hour),
	}

	req := httptest.NewRequest(http.MethodPut, "/api/status", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleStatusRejectsUnknownChannel(t *testing.T) {
	srv, err := newServer(config{
		traqBaseURL:        "https://example.test",
		traqBotAccessToken: "bot-token",
	})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.sessions["session-id"] = authSession{
		Token:     tokenResponse{AccessToken: "user-token"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v3/channels" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(r, `{"public":[{"id":"channel-a","name":"channel-a"}]}`), nil
	})}

	req := httptest.NewRequest(http.MethodPut, "/api/status", strings.NewReader(`{"channel":"missing-channel"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
