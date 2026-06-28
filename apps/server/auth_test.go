package main

import (
	"context"
	"encoding/json"
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
	srv.routes().ServeHTTP(rec, req)

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

func TestSessionTokenLoadsPersistedSession(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	expiresAt := time.Now().Add(time.Hour)
	srv.store = &fakePersistenceStore{
		sessions: map[string]authSession{
			"persisted-session": {
				Token:     tokenResponse{AccessToken: "persisted-access-token"},
				ExpiresAt: expiresAt,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "persisted-session"})

	token, ok := srv.sessionToken(req)
	if !ok {
		t.Fatal("sessionToken rejected persisted session")
	}
	if token.AccessToken != "persisted-access-token" {
		t.Fatalf("access token = %q, want persisted-access-token", token.AccessToken)
	}
	if _, ok := srv.sessions["persisted-session"]; !ok {
		t.Fatal("persisted session was not cached in memory")
	}
}

func TestSessionTokenDeletesExpiredPersistedSession(t *testing.T) {
	srv, err := newServer(config{})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	store := &fakePersistenceStore{
		sessions: map[string]authSession{
			"expired-persisted-session": {
				Token:     tokenResponse{AccessToken: "expired-access-token"},
				ExpiresAt: time.Now().Add(-time.Second),
			},
		},
	}
	srv.store = store

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-persisted-session"})

	if token, ok := srv.sessionToken(req); ok {
		t.Fatalf("sessionToken returned ok with token %#v, want expired session rejection", token)
	}
	if !store.deleted["expired-persisted-session"] {
		t.Fatal("expired persisted session was not deleted")
	}
}

type fakePersistenceStore struct {
	sessions map[string]authSession
	deleted  map[string]bool
}

func (s *fakePersistenceStore) SaveAuthSession(_ context.Context, sessionID string, session authSession) error {
	if s.sessions == nil {
		s.sessions = map[string]authSession{}
	}
	s.sessions[sessionID] = session
	return nil
}

func (s *fakePersistenceStore) FindAuthSession(_ context.Context, sessionID string) (authSession, bool, error) {
	session, ok := s.sessions[sessionID]
	return session, ok, nil
}

func (s *fakePersistenceStore) DeleteAuthSession(_ context.Context, sessionID string) error {
	if s.deleted == nil {
		s.deleted = map[string]bool{}
	}
	s.deleted[sessionID] = true
	delete(s.sessions, sessionID)
	return nil
}

func (s *fakePersistenceStore) DeleteExpiredAuthSessions(_ context.Context, now time.Time) error {
	for sessionID, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			_ = s.DeleteAuthSession(context.Background(), sessionID)
		}
	}
	return nil
}

func (s *fakePersistenceStore) LoadChannelScores(context.Context) (map[string]scoreRecord, error) {
	return nil, nil
}

func (s *fakePersistenceStore) SaveChannelScores(context.Context, []scoreRecord) error {
	return nil
}

func (s *fakePersistenceStore) Close() error {
	return nil
}

func TestHandleCallbackStoresTraqUserIDInSession(t *testing.T) {
	srv, err := newServer(config{
		appOrigin:        "http://localhost:5173",
		traqBaseURL:      "https://example.test",
		oauthClientID:    "client-id",
		oauthRedirectURL: "http://localhost:5173/oauth/callback",
	})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.states["state-id"] = time.Now().Add(time.Minute)
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/oauth2/token":
			return jsonResponse(r, `{"access_token":"access-token","token_type":"Bearer","expires_in":3600}`), nil
		case "/api/v3/users/me":
			return jsonResponse(r, `{"id":"user-id","name":"user_name"}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
			return nil, nil
		}
	})}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=code&state=state-id", nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d body=%q", rec.Code, http.StatusFound, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("cookies = %#v, want %s cookie", cookies, sessionCookieName)
	}
	session := srv.sessions[cookies[0].Value]
	if session.TraqUserID != "user-id" {
		t.Fatalf("TraqUserID = %q, want user-id", session.TraqUserID)
	}
	if session.TraqName != "user_name" {
		t.Fatalf("TraqName = %q, want user_name", session.TraqName)
	}
}

func TestEnsureSessionTraqUserIDUsesCachedValue(t *testing.T) {
	srv, err := newServer(config{traqBaseURL: "https://example.test"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
		return nil, nil
	})}
	session := authSession{
		Token:      tokenResponse{AccessToken: "access-token"},
		ExpiresAt:  time.Now().Add(time.Hour),
		TraqUserID: "cached-user",
	}

	userID, err := srv.ensureSessionTraqUserID(context.Background(), "session-id", session)
	if err != nil {
		t.Fatalf("ensureSessionTraqUserID returned error: %v", err)
	}
	if userID != "cached-user" {
		t.Fatalf("userID = %q, want cached-user", userID)
	}
}

func TestEnsureSessionTraqUserIDFetchesAndStoresMissingValue(t *testing.T) {
	srv, err := newServer(config{traqBaseURL: "https://example.test"})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	requests := 0
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Path != "/api/v3/users/me" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(r, `{"id":"fetched-user","name":"fetched_name"}`), nil
	})}
	srv.sessions["session-id"] = authSession{
		Token:     tokenResponse{AccessToken: "access-token"},
		ExpiresAt: time.Now().Add(time.Hour),
	}

	userID, err := srv.ensureSessionTraqUserID(context.Background(), "session-id", srv.sessions["session-id"])
	if err != nil {
		t.Fatalf("ensureSessionTraqUserID returned error: %v", err)
	}
	if userID != "fetched-user" {
		t.Fatalf("userID = %q, want fetched-user", userID)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if got := srv.sessions["session-id"].TraqUserID; got != "fetched-user" {
		t.Fatalf("stored TraqUserID = %q, want fetched-user", got)
	}
	if got := srv.sessions["session-id"].TraqName; got != "fetched_name" {
		t.Fatalf("stored TraqName = %q, want fetched_name", got)
	}
}

func TestHandleMeAddsTraqNameAndCachesUserInfo(t *testing.T) {
	srv, err := newServer(config{
		traqBaseURL:   "https://example.test",
		oauthClientID: "client-id",
	})
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	srv.sessions["session-id"] = authSession{
		Token:     tokenResponse{AccessToken: "access-token"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	requests := 0
	srv.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Path != "/api/v3/users/me" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(r, `{"id":"user-id","name":"traq_id"}`), nil
	})}

	for index := 0; index < 2; index++ {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
		rec := httptest.NewRecorder()
		srv.routes().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d body=%q", index, rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload meResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("request %d response is invalid JSON: %v", index, err)
		}
		if !payload.Authenticated {
			t.Fatalf("request %d authenticated = false, want true", index)
		}
		if !payload.OAuthConfigured {
			t.Fatalf("request %d oauthConfigured = false, want true", index)
		}
		if payload.Name != "traq_id" {
			t.Fatalf("request %d name = %q, want traq_id", index, payload.Name)
		}

		var raw map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("request %d response is invalid JSON object: %v", index, err)
		}
		if _, ok := raw["id"]; ok {
			t.Fatalf("request %d response contains unexpected id key: %v", index, raw)
		}
		if _, ok := raw["displayName"]; ok {
			t.Fatalf("request %d response contains unexpected displayName key: %v", index, raw)
		}
	}

	if requests != 1 {
		t.Fatalf("traQ users/me requests = %d, want 1", requests)
	}
	if got := srv.sessions["session-id"].TraqName; got != "traq_id" {
		t.Fatalf("stored TraqName = %q, want traq_id", got)
	}
}
