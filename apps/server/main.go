package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	loadDotEnv(".env")

	cfg := loadConfig()
	srv, err := newServer(cfg)
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}
	srv.startAuthCleanup(context.Background())
	defer srv.close()

	if cfg.oauthClientID == "" {
		traqLogWarn("TRAQ_CLIENT_ID is empty; live mode OAuth is disabled")
	}
	preloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.preloadLiveChannelData(preloadCtx); err != nil {
		log.Fatalf("preload live channel data: %v", err)
	}

	log.Println("起動しました")
	if err := srv.routes().Start(cfg.addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
