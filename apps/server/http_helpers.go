package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.allowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) allowedOrigin(origin string) bool {
	if origin == "" || origin == s.cfg.appOrigin {
		return origin != ""
	}
	want, err := url.Parse(s.cfg.appOrigin)
	if err != nil {
		return false
	}
	got, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return want.Scheme == got.Scheme && want.Port() == got.Port() &&
		isLoopback(want.Hostname()) && isLoopback(got.Hostname())
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func marshalEvent(name string, value any) sseEvent {
	data, err := json.Marshal(value)
	if err != nil {
		data = []byte(`{"error":"marshal failed"}`)
	}
	return sseEvent{Name: name, Data: data}
}

func writeSSE(w io.Writer, event sseEvent) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Name)
	for _, line := range strings.Split(string(event.Data), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}
