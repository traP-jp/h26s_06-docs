package main

import (
	"log"
	"net/http"
)

func main() {
	loadDotEnv(".env")

	cfg := loadConfig()
	srv, err := newServer(cfg)
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}
	defer srv.close()

	if cfg.oauthClientID == "" {
		traqLogWarn("TRAQ_CLIENT_ID is empty; live mode OAuth is disabled")
	}
	if cfg.traqBotAccessToken == "" {
		traqLogWarn("TRAQ_BOT_ACCESS_TOKEN is empty; viewer polling is disabled")
	}
	if err := http.ListenAndServe(cfg.addr, srv.routes()); err != nil {
		log.Fatal(err)
	}
}
