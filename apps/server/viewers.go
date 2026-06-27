package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type viewerPoller struct {
	mu            sync.Mutex
	channels      []traqChannel
	messageWeight map[string]float64
	maxPerTick    int
}

type weightedChannel struct {
	channel traqChannel
	weight  float64
}

func newViewerPoller(channels []traqChannel, maxPerTick int) *viewerPoller {
	active := make([]traqChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.ID != "" && !ch.Archived {
			active = append(active, ch)
		}
	}
	if maxPerTick <= 0 || maxPerTick > len(active) {
		maxPerTick = len(active)
	}
	return &viewerPoller{channels: active, maxPerTick: maxPerTick, messageWeight: map[string]float64{}}
}

func (p *viewerPoller) noteMessage(channelID string) {
	if p == nil || channelID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messageWeight[channelID] = math.Min(120, p.messageWeight[channelID]+12)
}

func (p *viewerPoller) sampleChannels() []traqChannel {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.channels) <= p.maxPerTick {
		return append([]traqChannel(nil), p.channels...)
	}

	candidates := make([]weightedChannel, 0, len(p.channels))
	for _, ch := range p.channels {
		weight := 1 + p.messageWeight[ch.ID]
		candidates = append(candidates, weightedChannel{channel: ch, weight: weight})
		if p.messageWeight[ch.ID] < 0.05 {
			delete(p.messageWeight, ch.ID)
		} else {
			p.messageWeight[ch.ID] *= 0.82
		}
	}

	selected := make([]traqChannel, 0, p.maxPerTick)
	for len(selected) < p.maxPerTick && len(candidates) > 0 {
		total := 0.0
		for _, c := range candidates {
			total += c.weight
		}
		pick := rand.Float64() * total
		index := 0
		for i, c := range candidates {
			pick -= c.weight
			if pick <= 0 {
				index = i
				break
			}
		}
		selected = append(selected, candidates[index].channel)
		candidates = append(candidates[:index], candidates[index+1:]...)
	}
	return selected
}

func (s *server) streamViewerSnapshots(ctx context.Context, accessToken string, poller *viewerPoller) <-chan viewerSnapshotPayload {
	out := make(chan viewerSnapshotPayload)
	go func() {
		defer close(out)
		ticker := time.NewTicker(s.cfg.viewerPollInterval)
		defer ticker.Stop()

		for {
			snapshot, err := s.fetchViewerSnapshot(ctx, accessToken, poller)
			if err == nil {
				select {
				case out <- snapshot:
				case <-ctx.Done():
					return
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/channels/"+url.PathEscape(channelID)+"/viewers", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("channel viewers endpoint returned %s for %s: %s", resp.Status, channelID, strings.TrimSpace(string(body)))
	}

	var viewers []traqViewer
	if err := json.Unmarshal(body, &viewers); err != nil {
		return nil, err
	}
	return viewers, nil
}
