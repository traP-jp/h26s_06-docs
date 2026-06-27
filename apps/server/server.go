package main

import (
	"context"
	"net/http"
	"time"
)

func newServer(cfg config) (*server, error) {
	demoState, err := newDemoStateManager()
	if err != nil {
		return nil, err
	}

	return &server{
		cfg:        cfg,
		client:     &http.Client{Timeout: 15 * time.Second},
		states:     map[string]time.Time{},
		sessions:   map[string]sessionRecord{},
		demoState:  demoState,
		demoHub:    newEventHub(),
		liveHub:    newEventHub(),
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
	if s.authCleanupCancel != nil {
		s.authCleanupCancel()
	}
	s.demoHub.close()
	s.liveHub.close()
}

func (s *server) startAuthCleanup(ctx context.Context) {
	cleanupCtx, cancel := context.WithCancel(ctx)
	s.authCleanupCancel = cancel

	go func() {
		ticker := time.NewTicker(authCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
				s.cleanupExpiredAuth(time.Now())
			}
		}
	}()
}

func (s *server) startDemoProducer() {
	s.demoOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.demoCancel = cancel
		go s.runDemoProducer(ctx, s.demoState, s.demoHub)
	})
}

func (s *server) ensureLiveChannelData(ctx context.Context, accessToken string) (channelData, error) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	if s.liveReady {
		return s.liveData, nil
	}

	data, err := s.fetchChannelData(ctx, accessToken)
	if err != nil {
		return channelData{}, err
	}
	s.liveData = data
	s.liveReady = true
	return data, nil
}
