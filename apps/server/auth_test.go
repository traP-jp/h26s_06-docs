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

func TestSessionTokenRejectsExpiredSession(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}

	srv.sessions["expired-session"] = authSession{
		Token:     tokenResponse{AccessToken: "expired-access-token"},
		ExpiresAt: time.Now().Add(-time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-session"})

	if token, ok := srv.sessionToken(req); ok {
		t.Fatalf("sessionToken returned ok with token %#v, want expired session rejection", token)
	}
	if _, ok := srv.sessions["expired-session"]; ok {
		t.Fatal("expired session was not removed")
	}
}
