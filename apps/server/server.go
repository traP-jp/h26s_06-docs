package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const authCleanupInterval = 10 * time.Minute

func newServer(cfg config) (*server, error) {
	if cfg.mariaDB.incomplete() {
		return nil, fmt.Errorf("incomplete MariaDB config: missing %s", strings.Join(cfg.mariaDB.missing, ", "))
	}

	var store persistenceStore
	if cfg.mariaDB.enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		mariaDBStore, err := openMariaDBStore(ctx, cfg.mariaDB)
		if err != nil {
			return nil, fmt.Errorf("connect MariaDB: %w", err)
		}
		store = mariaDBStore
		traqLogOK("MariaDB persistence enabled")
	}

	demoState, err := newDemoStateManager()
	if err != nil {
		if store != nil {
			_ = store.Close()
		}
		return nil, err
	}

	return &server{
		cfg:          cfg,
		client:       &http.Client{Timeout: 15 * time.Second},
		store:        store,
		states:       map[string]time.Time{},
		sessions:     map[string]authSession{},
		userBotCache: map[string]bool{},
		demoState:    demoState,
		demoHub:      newEventHub(),
		liveHub:      newEventHub(),
		viewerHub:    newViewerSignalHub(),
		initTokens:   make(chan struct{}, maxConcurrentInits),
	}, nil
}

func (s *server) routes() *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			return s.allowedOrigin(origin), nil
		},
		AllowHeaders:     []string{echo.HeaderContentType},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
		AllowCredentials: true,
	}))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	})

	methods := []string{http.MethodGet, http.MethodPost}
	e.Match(methods, "/api/auth/login", s.handleLogin)
	e.Match(methods, "/api/auth/callback", s.handleCallback)
	e.Match(methods, "/api/auth/logout", s.handleLogout)
	e.Match(methods, "/api/events", s.handleEvents)
	e.Match(methods, "/api/me", s.handleMe)
	e.PUT("/api/status", s.handleStatus)
	e.Match(methods, "/healthz", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	return e
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
	s.liveMu.Lock()
	liveState := s.liveData.State
	s.liveMu.Unlock()
	if liveState != nil {
		s.persistChannelScores(liveState)
	}
	s.demoHub.close()
	s.liveHub.close()
	s.viewerHub.close()
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			traqLogWarn("close persistence store: %v", err)
		}
	}
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
		go s.runLiveSyncProducer(ctx, state, s.liveHub)
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
