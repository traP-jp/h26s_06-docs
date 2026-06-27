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
		cfg:          cfg,
		client:       &http.Client{Timeout: 15 * time.Second},
		states:       map[string]time.Time{},
		sessions:     map[string]authSession{},
		userBotCache: map[string]bool{},
		demoState:    demoState,
		demoHub:      newEventHub(),
		liveHub:      newEventHub(),
		initTokens:   make(chan struct{}, maxConcurrentInits),
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
	if s.liveViewersCancel != nil {
		s.liveViewersCancel()
	}
	if s.demoSyncCancel != nil {
		s.demoSyncCancel()
	}
	if s.liveSyncCancel != nil {
		s.liveSyncCancel()
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

func (s *server) startDemoSyncProducer() {
	s.demoSyncOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.demoSyncCancel = cancel
		go s.runSyncProducer(ctx, s.demoState, s.demoHub)
	})
}

func (s *server) startLiveSyncProducer(state *stateManager) {
	s.liveSyncOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.liveSyncCancel = cancel
		go s.runSyncProducer(ctx, state, s.liveHub)
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

func (s *server) preloadLiveChannelData(ctx context.Context) error {
	if s.cfg.traqBotAccessToken == "" {
		traqLogWarn("TRAQ_BOT_ACCESS_TOKEN is empty; live channel tree preload and viewer polling are disabled")
		return nil
	}
	data, err := s.ensureLiveChannelData(ctx, s.cfg.traqBotAccessToken)
	if err != nil {
		return err
	}
	traqLogOK("live channel tree preloaded channels=%d", len(data.Channels))
	return nil
}

func (s *server) ensureLiveChannelData(ctx context.Context, accessToken string) (channelData, error) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	if s.liveReady {
		return s.liveData, nil
	}

	data, err := s.fetchChannelData(ctx, s.cfg.traqBotAccessToken)
	if err != nil {
		return channelData{}, err
	}
	s.liveData = data
	s.liveReady = true
	return data, nil
}
