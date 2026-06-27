package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// traqGetJSON makes a GET request to the traQ API at path using the given bearer token,
// reads up to 1 MB of the response body, checks for a 2xx status, and unmarshals JSON
// into out. Returns the HTTP status code so callers can handle specific codes (e.g. 404).
func (s *server) traqGetJSON(ctx context.Context, accessToken, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return resp.StatusCode, json.Unmarshal(body, out)
}
