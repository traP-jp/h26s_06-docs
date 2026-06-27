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
		sessions:   map[string]tokenResponse{},
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
	if s.liveViewersCancel != nil {
		s.liveViewersCancel()
	}
	s.demoHub.close()
	s.liveHub.close()
}

func (s *server) startDemoProducer() {
	s.demoOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.demoCancel = cancel
		go s.runDemoProducer(ctx, s.demoState, s.demoHub)
	})
}

func (s *server) startLiveViewerPolling(channels []traqChannel, state *stateManager) {
	s.liveViewersOnce.Do(func() {
		if s.cfg.traqBotAccessToken == "" {
			traqLogWarn("TRAQ_BOT_ACCESS_TOKEN is empty; viewer polling is disabled")
			return
		}
		traqLogOK("viewer polling started with bot token channels=%d interval=%s", len(channels), s.cfg.viewerPollInterval)
		ctx, cancel := context.WithCancel(context.Background())
		s.liveViewersCancel = cancel
		go s.consumeViewerSnapshots(ctx, s.cfg.traqBotAccessToken, channels, state, s.liveHub)
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
