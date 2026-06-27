package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

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

func echoHTTPError(c echo.Context, message string, status int) error {
	http.Error(c.Response().Writer, message, status)
	return nil
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
