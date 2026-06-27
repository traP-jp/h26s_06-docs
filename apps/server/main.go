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

	log.Printf("listening on %s", cfg.addr)
	if cfg.oauthClientID == "" {
		log.Printf("TRAQ_CLIENT_ID is empty; live mode OAuth is disabled")
	}
	if err := http.ListenAndServe(cfg.addr, logRequests(srv.routes())); err != nil {
		log.Fatal(err)
	}
}
