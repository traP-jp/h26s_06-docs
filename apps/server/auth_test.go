package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestHandleLoginUsesAuthorizationCodeFlow(t *testing.T) {
	srv, err := newServer(config{
		appOrigin:        "http://localhost:5173",
		traqBaseURL:      "https://q.trap.jp",
		oauthClientID:    "client-id",
		oauthRedirectURL: "http://localhost:5173/oauth/callback",
		oauthScope:       "read",
	})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	rec := httptest.NewRecorder()
	srv.handleLogin(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location is invalid: %v", err)
	}
	values := location.Query()
	if values.Get("response_type") != "code" {
		t.Fatalf("response_type = %q, want code", values.Get("response_type"))
	}
	if values.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q, want client-id", values.Get("client_id"))
	}
	if values.Get("state") == "" {
		t.Fatal("state was empty")
	}
}

func TestSessionTokenRemovesExpiredSession(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.sessions["expired"] = sessionRecord{
		token:     tokenResponse{AccessToken: "token"},
		expiresAt: time.Now().Add(-time.Minute),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired"})

	if _, ok := srv.sessionToken(req); ok {
		t.Fatal("expired session was accepted")
	}
	if _, ok := srv.sessions["expired"]; ok {
		t.Fatal("expired session was not removed")
	}
}

func TestSessionTokenAcceptsUnexpiredSession(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.sessions["active"] = sessionRecord{
		token:     tokenResponse{AccessToken: "token"},
		expiresAt: time.Now().Add(time.Minute),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "active"})

	token, ok := srv.sessionToken(req)
	if !ok {
		t.Fatal("active session was rejected")
	}
	if token.AccessToken != "token" {
		t.Fatalf("access token = %q, want token", token.AccessToken)
	}
}

func TestCleanupExpiredAuthRemovesExpiredStatesAndSessions(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	now := time.Now()
	srv.states["expired"] = now.Add(-time.Minute)
	srv.states["active"] = now.Add(time.Minute)
	srv.sessions["expired"] = sessionRecord{expiresAt: now.Add(-time.Minute)}
	srv.sessions["active"] = sessionRecord{expiresAt: now.Add(time.Minute)}

	srv.cleanupExpiredAuth(now)

	if _, ok := srv.states["expired"]; ok {
		t.Fatal("expired state was not removed")
	}
	if _, ok := srv.sessions["expired"]; ok {
		t.Fatal("expired session was not removed")
	}
	if _, ok := srv.states["active"]; !ok {
		t.Fatal("active state was removed")
	}
	if _, ok := srv.sessions["active"]; !ok {
		t.Fatal("active session was removed")
	}
}
