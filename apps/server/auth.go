package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type traqMe struct {
	Name string `json:"name"`
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.oauthClientID == "" {
		http.Error(w, "TRAQ_CLIENT_ID is not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "failed to create state", http.StatusInternalServerError)
		return
	}

	s.authMu.Lock()
	s.states[state] = time.Now().Add(10 * time.Minute)
	s.authMu.Unlock()

	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", s.cfg.oauthClientID)
	values.Set("redirect_uri", s.cfg.oauthRedirectURL)
	values.Set("scope", s.cfg.oauthScope)
	values.Set("state", state)

	http.Redirect(w, r, s.cfg.traqBaseURL+"/api/v3/oauth2/authorize?"+values.Encode(), http.StatusFound)
}

func (s *server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, s.cfg.appOrigin, http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	if !s.consumeOAuthState(state) {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	token, err := s.exchangeAuthorizationCode(r.Context(), code)
	if err != nil {
		traqLogError("oauth token exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}

	if len(s.cfg.allowedTraqIDs) > 0 {
		me, err := s.fetchTraqMe(r.Context(), token.AccessToken)
		if err != nil {
			traqLogError("failed to fetch traQ user info: %v", err)
			http.Error(w, "failed to verify user identity", http.StatusBadGateway)
			return
		}
		if !s.cfg.allowedTraqIDs[me.Name] {
			http.Redirect(w, r, s.cfg.appOrigin+"?error=forbidden", http.StatusFound)
			return
		}
	}

	sessionID, err := randomToken(32)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	s.authMu.Lock()
	s.sessions[sessionID] = token
	s.authMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   max(token.ExpiresIn, int(time.Hour.Seconds())),
	})
	http.Redirect(w, r, s.cfg.appOrigin, http.StatusFound)
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	_, ok := s.sessionToken(r)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"authenticated":   ok,
		"oauthConfigured": s.cfg.oauthClientID != "",
	})
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.authMu.Lock()
		delete(s.sessions, cookie.Value)
		s.authMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) exchangeAuthorizationCode(ctx context.Context, code string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", s.cfg.oauthClientID)
	values.Set("redirect_uri", s.cfg.oauthRedirectURL)
	values.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.traqBaseURL+"/api/v3/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("token endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return tokenResponse{}, err
	}
	if token.AccessToken == "" {
		return tokenResponse{}, errors.New("token endpoint did not return access_token")
	}
	return token, nil
}

func (s *server) consumeOAuthState(state string) bool {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	expiresAt, ok := s.states[state]
	delete(s.states, state)
	return ok && time.Now().Before(expiresAt)
}

func (s *server) sessionToken(r *http.Request) (tokenResponse, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return tokenResponse{}, false
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	token, ok := s.sessions[cookie.Value]
	return token, ok
}

func (s *server) fetchTraqMe(ctx context.Context, accessToken string) (traqMe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/users/me", nil)
	if err != nil {
		return traqMe{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return traqMe{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return traqMe{}, fmt.Errorf("users/me returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var me traqMe
	if err := json.Unmarshal(body, &me); err != nil {
		return traqMe{}, err
	}
	if me.Name == "" {
		return traqMe{}, errors.New("users/me did not return name")
	}
	return me, nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
