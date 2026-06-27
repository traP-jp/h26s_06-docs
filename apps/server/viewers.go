package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type viewerPoller struct {
	channels   []traqChannel
	maxPerTick int
	state      *stateManager
}

type weightedChannel struct {
	id               string
	channel          traqChannel
	rawWeight        float64
	normalizedWeight float64
}

func newViewerPoller(channels []traqChannel, maxPerTick int, state *stateManager) *viewerPoller {
	active := make([]traqChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.ID != "" && !ch.Archived {
			active = append(active, ch)
		}
	}
	if maxPerTick <= 0 || maxPerTick > len(active) {
		maxPerTick = len(active)
	}
	return &viewerPoller{channels: active, maxPerTick: maxPerTick, state: state}
}

func (p *viewerPoller) sampleChannels() []traqChannel {
	if p == nil || p.state == nil {
		return nil
	}
	selected := p.state.sampleViewerChannels(p.channels, p.maxPerTick)
	traqLogAPI("viewer poll selected channels=%d candidates=%d max=%d", len(selected), len(p.channels), p.maxPerTick)
	return selected
}

func selectWeightedChannels(candidates []weightedChannel, maxChannels int) []weightedChannel {
	if maxChannels <= 0 || len(candidates) == 0 {
		return nil
	}
	candidates = normalizeWeightedChannels(candidates)
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) <= maxChannels {
		return candidates
	}
	selected := make([]weightedChannel, 0, maxChannels)
	for len(selected) < maxChannels && len(candidates) > 0 {
		total := 0.0
		for _, c := range candidates {
			total += c.normalizedWeight
		}
		pick := rand.Float64() * total
		index := 0
		for i, c := range candidates {
			pick -= c.normalizedWeight
			if pick <= 0 {
				index = i
				break
			}
		}
		selected = append(selected, candidates[index])
		candidates = append(candidates[:index], candidates[index+1:]...)
	}
	return selected
}

func normalizeWeightedChannels(candidates []weightedChannel) []weightedChannel {
	total := 0.0
	for _, candidate := range candidates {
		if candidate.rawWeight > 0 {
			total += candidate.rawWeight
		}
	}
	if total <= 0 {
		return nil
	}
	normalized := make([]weightedChannel, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.rawWeight <= 0 {
			continue
		}
		candidate.normalizedWeight = candidate.rawWeight / total
		normalized = append(normalized, candidate)
	}
	return normalized
}

func (s *server) streamViewerSnapshots(ctx context.Context, accessToken string, poller *viewerPoller, hub *eventHub) <-chan viewerSnapshotPayload {
	out := make(chan viewerSnapshotPayload)
	go func() {
		defer close(out)
		ticker := time.NewTicker(s.cfg.viewerPollInterval)
		defer ticker.Stop()

		for {
			if hub == nil || hub.hasSubscribers() {
				snapshot, err := s.fetchViewerSnapshot(ctx, accessToken, poller)
				if err == nil {
					traqLogOK(
						"viewer snapshot sampled=%d totalChannels=%d rows=%d summaries=%d",
						snapshot.SampledChannels,
						snapshot.TotalChannels,
						snapshot.Total,
						len(snapshot.Channels),
					)
					select {
					case out <- snapshot:
					case <-ctx.Done():
						return
					}
				} else if ctx.Err() == nil {
					traqLogWarn("viewer snapshot skipped: %v", err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return out
}

func (s *server) fetchViewerSnapshot(ctx context.Context, accessToken string, poller *viewerPoller) (viewerSnapshotPayload, error) {
	channels := poller.sampleChannels()
	type result struct {
		channel traqChannel
		viewers []traqViewer
		err     error
	}

	sem := make(chan struct{}, 8)
	results := make(chan result, len(channels))
	var wg sync.WaitGroup
	for _, channel := range channels {
		channel := channel
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- result{channel: channel, err: ctx.Err()}
				return
			}
			viewers, err := s.fetchChannelViewers(ctx, accessToken, channel.ID)
			results <- result{channel: channel, viewers: viewers, err: err}
		}()
	}
	wg.Wait()
	close(results)

	summaries := make([]viewerChannelSummary, 0)
	rows := make([]viewerRow, 0)
	var firstErr error
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		if len(res.viewers) == 0 {
			continue
		}
		summary := viewerChannelSummary{ChannelID: res.channel.ID, ChannelName: res.channel.Name, Count: len(res.viewers)}
		for _, viewer := range res.viewers {
			switch viewer.State {
			case "editing":
				summary.Editing++
			case "monitoring":
				summary.Monitoring++
			case "stale_viewing":
				summary.Stale++
			}
			rows = append(rows, viewerRow{
				UserID:      viewer.UserID,
				ChannelID:   res.channel.ID,
				ChannelName: res.channel.Name,
				State:       viewer.State,
				UpdatedAt:   viewer.UpdatedAt,
			})
		}
		summaries = append(summaries, summary)
	}
	if len(rows) == 0 && firstErr != nil {
		return viewerSnapshotPayload{}, firstErr
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Count == summaries[j].Count {
			return summaries[i].ChannelName < summaries[j].ChannelName
		}
		return summaries[i].Count > summaries[j].Count
	})
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})
	if len(summaries) > 12 {
		summaries = summaries[:12]
	}
	if len(rows) > 24 {
		rows = rows[:24]
	}

	return viewerSnapshotPayload{
		TS:              time.Now().Unix(),
		Total:           len(rows),
		SampledChannels: len(channels),
		TotalChannels:   len(poller.channels),
		Channels:        summaries,
		Recent:          rows,
	}, nil
}

func (s *server) fetchChannelViewers(ctx context.Context, accessToken string, channelID string) ([]traqViewer, error) {
	traqLogAPI("GET /api/v3/channels/%s/viewers", channelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/channels/"+url.PathEscape(channelID)+"/viewers", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		traqLogError("GET /api/v3/channels/%s/viewers failed: %v", channelID, err)
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		traqLogWarn("GET /api/v3/channels/%s/viewers -> %s", channelID, resp.Status)
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		traqLogError("GET /api/v3/channels/%s/viewers -> %s", channelID, resp.Status)
		return nil, fmt.Errorf("channel viewers endpoint returned %s for %s: %s", resp.Status, channelID, strings.TrimSpace(string(body)))
	}

	var viewers []traqViewer
	if err := json.Unmarshal(body, &viewers); err != nil {
		return nil, err
	}
	traqLogOK("GET /api/v3/channels/%s/viewers -> %s viewers=%d", channelID, resp.Status, len(viewers))
	return viewers, nil
}
