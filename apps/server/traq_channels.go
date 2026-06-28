package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type traqChannelList struct {
	Public []traqChannel `json:"public"`
}

func (s *server) fetchChannelData(ctx context.Context, accessToken string) (channelData, error) {
	traqLogAPI("GET /api/v3/channels")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/channels", nil)
	if err != nil {
		return channelData{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		traqLogError("GET /api/v3/channels failed: %v", err)
		return channelData{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		traqLogError("GET /api/v3/channels -> %s", resp.Status)
		return channelData{}, fmt.Errorf("channels endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var list traqChannelList
	if err := json.Unmarshal(body, &list); err != nil {
		return channelData{}, err
	}
	channels := activeTraqChannels(list.Public)
	state, err := newStateManagerFromTraq(channels)
	if err != nil {
		return channelData{}, err
	}
	traqLogOK("GET /api/v3/channels -> %s public=%d active=%d", resp.Status, len(list.Public), len(channels))
	return channelData{
		Channels:   channels,
		ChannelIDs: channelIDSet(channels),
		State:      state,
	}, nil
}

func activeTraqChannels(channels []traqChannel) []traqChannel {
	byID := make(map[string]traqChannel, len(channels))
	for _, ch := range channels {
		if ch.ID != "" {
			byID[ch.ID] = ch
		}
	}

	cache := make(map[string]bool, len(channels))
	active := make([]traqChannel, 0, len(channels))
	for _, ch := range channels {
		if channelAndAncestorsActive(ch.ID, byID, cache, map[string]bool{}) {
			active = append(active, ch)
		}
	}
	return active
}

func channelAndAncestorsActive(id string, channels map[string]traqChannel, cache map[string]bool, visiting map[string]bool) bool {
	if id == "" {
		return false
	}
	if active, ok := cache[id]; ok {
		return active
	}
	if visiting[id] {
		cache[id] = false
		return false
	}
	ch, ok := channels[id]
	if !ok || ch.Archived {
		cache[id] = false
		return false
	}
	if ch.ParentID == nil || *ch.ParentID == "" {
		cache[id] = true
		return true
	}
	visiting[id] = true
	active := channelAndAncestorsActive(*ch.ParentID, channels, cache, visiting)
	delete(visiting, id)
	cache[id] = active
	return active
}

func channelIDSet(channels []traqChannel) map[string]bool {
	ids := make(map[string]bool, len(channels))
	for _, ch := range channels {
		if ch.ID != "" && !ch.Archived {
			ids[ch.ID] = true
		}
	}
	return ids
}
