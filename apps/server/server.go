package main

import (
	"context"
	"net/http"
	"time"
)

func newServer(cfg config) (*server, error) {
	state, err := newDemoStateManager()
	if err != nil {
		return nil, err
	}

	return &server{
		cfg:        cfg,
		client:     &http.Client{Timeout: 15 * time.Second},
		states:     map[string]time.Time{},
		sessions:   map[string]tokenResponse{},
		state:      state,
		hub:        newEventHub(),
		initTokens: make(chan struct{}, maxConcurrentInits),
	}, nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/callback", s.handleCallback)
	mux.HandleFunc("/api/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/me", s.handleMe)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return s.withCORS(mux)
}

func (s *server) close() {
	if s.demoCancel != nil {
		s.demoCancel()
	}
	s.hub.close()
}

func (s *server) startDemoProducer() {
	s.demoOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.demoCancel = cancel
		go s.runDemoProducer(ctx)
	})
}

func (s *server) currentState() *stateManager {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *server) replaceState(state *stateManager) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state = state
}
