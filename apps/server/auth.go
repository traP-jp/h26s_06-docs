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

	"github.com/labstack/echo/v4"
)

const (
	sessionCookieName = "traq_session"
)

type authSession struct {
	Token      tokenResponse
	ExpiresAt  time.Time
	TraqUserID string
	TraqName   string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type traqMe struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type meResponse struct {
	Authenticated   bool   `json:"authenticated"`
	OAuthConfigured bool   `json:"oauthConfigured"`
	Name            string `json:"name,omitempty"`
}

func (s *server) handleLogin(c echo.Context) error {
	if s.cfg.oauthClientID == "" {
		return echoHTTPError(c, "TRAQ_CLIENT_ID is not configured", http.StatusServiceUnavailable)
	}

	state, err := randomToken(32)
	if err != nil {
		return echoHTTPError(c, "failed to create state", http.StatusInternalServerError)
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

	return c.Redirect(http.StatusFound, s.cfg.traqBaseURL+"/api/v3/oauth2/authorize?"+values.Encode())
}

func (s *server) handleCallback(c echo.Context) error {
	r := c.Request()
	if r.URL.Query().Get("error") != "" {
		return c.Redirect(http.StatusFound, s.cfg.appOrigin)
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		return echoHTTPError(c, "missing code or state", http.StatusBadRequest)
	}
	if !s.consumeOAuthState(state) {
		return echoHTTPError(c, "invalid state", http.StatusBadRequest)
	}

	token, err := s.exchangeAuthorizationCode(r.Context(), code)
	if err != nil {
		traqLogError("oauth token exchange failed: %v", err)
		return echoHTTPError(c, "token exchange failed", http.StatusBadGateway)
	}

	me, err := s.fetchTraqMe(r.Context(), token.AccessToken)
	if err != nil {
		traqLogError("failed to fetch traQ user info: %v", err)
		return echoHTTPError(c, "failed to fetch user info", http.StatusBadGateway)
	}
	if len(s.cfg.allowedTraqIDs) > 0 {
		if !s.cfg.allowedTraqIDs[me.Name] {
			return c.Redirect(http.StatusFound, s.cfg.appOrigin+"?error=forbidden")
		}
	}
	traqUserID := me.userID()

	sessionID, err := randomToken(32)
	if err != nil {
		return echoHTTPError(c, "failed to create session", http.StatusInternalServerError)
	}

	sessionMaxAge := token.ExpiresIn
	sessionExpiresAt := time.Now().Add(time.Duration(sessionMaxAge) * time.Second)

	s.authMu.Lock()
	s.sessions[sessionID] = authSession{
		Token:      token,
		ExpiresAt:  sessionExpiresAt,
		TraqUserID: traqUserID,
		TraqName:   me.traqName(),
	}
	s.authMu.Unlock()

	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAge,
	})
	return c.Redirect(http.StatusFound, s.cfg.appOrigin)
}

func (s *server) handleMe(c echo.Context) error {
	sessionID, session, ok := s.session(c.Request())
	if !ok {
		return c.JSON(http.StatusOK, meResponse{
			Authenticated:   false,
			OAuthConfigured: s.cfg.oauthClientID != "",
		})
	}

	me, err := s.ensureSessionTraqMe(c.Request().Context(), sessionID, session)
	if err != nil {
		traqLogError("failed to fetch traQ user info for /api/me: %v", err)
		return echoHTTPError(c, "failed to fetch user info", http.StatusBadGateway)
	}

	return c.JSON(http.StatusOK, meResponse{
		Authenticated:   true,
		OAuthConfigured: s.cfg.oauthClientID != "",
		Name:            me.traqName(),
	})
}

func (s *server) handleLogout(c echo.Context) error {
	r := c.Request()
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.authMu.Lock()
		delete(s.sessions, cookie.Value)
		s.authMu.Unlock()
	}
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return c.NoContent(http.StatusNoContent)
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
	_, session, ok := s.session(r)
	if !ok {
		return tokenResponse{}, false
	}
	return session.Token, true
}

func (s *server) session(r *http.Request) (string, authSession, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", authSession{}, false
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	session, ok := s.sessions[cookie.Value]
	if !ok {
		return "", authSession{}, false
	}
	if !time.Now().Before(session.ExpiresAt) {
		delete(s.sessions, cookie.Value)
		return "", authSession{}, false
	}
	return cookie.Value, session, true
}

func (s *server) ensureSessionTraqUserID(ctx context.Context, sessionID string, session authSession) (string, error) {
	if session.TraqUserID != "" {
		return session.TraqUserID, nil
	}

	me, err := s.ensureSessionTraqMe(ctx, sessionID, session)
	if err != nil {
		return "", err
	}
	return me.userID(), nil
}

func (s *server) ensureSessionTraqMe(ctx context.Context, sessionID string, session authSession) (traqMe, error) {
	if me, ok := session.cachedTraqMe(); ok {
		return me, nil
	}

	s.authMu.Lock()
	current, ok := s.sessions[sessionID]
	if ok && time.Now().Before(current.ExpiresAt) {
		if me, cached := current.cachedTraqMe(); cached {
			s.authMu.Unlock()
			return me, nil
		}
	}
	s.authMu.Unlock()

	me, err := s.fetchTraqMe(ctx, session.Token.AccessToken)
	if err != nil {
		return traqMe{}, err
	}

	s.authMu.Lock()
	defer s.authMu.Unlock()
	current, ok = s.sessions[sessionID]
	if !ok || !time.Now().Before(current.ExpiresAt) {
		return me, nil
	}
	if cached, ok := current.cachedTraqMe(); ok {
		return cached, nil
	}
	current.TraqUserID = me.userID()
	current.TraqName = me.traqName()
	s.sessions[sessionID] = current
	return me, nil
}

func (s *server) cleanupExpiredAuth(now time.Time) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	for state, expiresAt := range s.states {
		if !now.Before(expiresAt) {
			delete(s.states, state)
		}
	}
	for sessionID, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}

func (s *server) fetchTraqMe(ctx context.Context, accessToken string) (traqMe, error) {
	var me traqMe
	if _, err := s.traqGetJSON(ctx, accessToken, "/api/v3/users/me", &me); err != nil {
		return traqMe{}, fmt.Errorf("users/me: %w", err)
	}
	if me.ID == "" && me.Name == "" {
		return traqMe{}, errors.New("users/me did not return id or name")
	}
	return me, nil
}

func (me traqMe) userID() string {
	if me.ID != "" {
		return me.ID
	}
	return me.Name
}

func (me traqMe) traqName() string {
	return me.Name
}

func (session authSession) cachedTraqMe() (traqMe, bool) {
	if session.TraqName == "" {
		return traqMe{}, false
	}
	return traqMe{ID: session.TraqUserID, Name: session.TraqName}, true
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
