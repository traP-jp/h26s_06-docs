package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	demo := r.URL.Query().Get("demo") == "1"
	var token tokenResponse

	if !demo {
		var ok bool
		token, ok = s.sessionToken(r)
		if !ok {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	streamState := s.demoState
	streamHub := s.demoHub
	initPayload := streamState.initPayloadBytes()
	var liveChannelIDs map[string]bool
	var liveChannels []traqChannel

	if !demo {
		data, err := s.ensureLiveChannelData(r.Context(), token.AccessToken)
		if err != nil {
			writeSSE(w, marshalEvent("stream-error", map[string]string{"error": err.Error()}))
			flusher.Flush()
			return
		}
		streamState = data.State
		streamHub = s.liveHub
		initPayload = data.InitJSON
		liveChannelIDs = data.ChannelIDs
		liveChannels = data.Channels
	}

	select {
	case s.initTokens <- struct{}{}:
		writeSSE(w, sseEvent{Name: "init", Data: initPayload})
		flusher.Flush()
		<-s.initTokens
	case <-r.Context().Done():
		return
	}

	events := streamHub.subscribe()
	defer streamHub.unsubscribe(events)

	if !demo {
		s.startLiveViewerPolling(liveChannels, streamState)
	}

	writeSSE(w, marshalEvent("status", map[string]string{"status": streamStatus(demo)}))
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if demo {
		s.startDemoProducer()
	} else {
		go s.consumeTraqStream(ctx, token.AccessToken, liveChannelIDs, streamState, streamHub)
	}

	syncTicker := time.NewTicker(s.cfg.syncInterval)
	heartbeat := time.NewTicker(25 * time.Second)
	defer syncTicker.Stop()
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case <-syncTicker.C:
			payload := streamState.syncPayload()
			if len(payload.Deltas) > 0 {
				writeSSE(w, marshalEvent("sync", payload))
				flusher.Flush()
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func streamStatus(demo bool) string {
	if demo {
		return "demo connected"
	}
	return "traQ connected"
}

func (s *server) publishTrigger(trigger triggerPayload, activeChannelIDs map[string]bool, state *stateManager, hub *eventHub) {
	if activeChannelIDs != nil && !triggerInActiveChannels(trigger, activeChannelIDs) {
		if trigger.Type == "mov" {
			debugMov(trigger, "", "", "skipped", "destination channel is not in active channel set", 0)
		}
		return
	}
	applied, changed := state.applyTrigger(trigger)
	if !changed {
		return
	}
	hub.publish(marshalEvent("trigger", applied))
}

func triggerInActiveChannels(trigger triggerPayload, active map[string]bool) bool {
	switch trigger.Type {
	case "msg":
		return active[trigger.Ch]
	case "mov":
		return active[trigger.To]
	default:
		return false
	}
}

func (s *server) runDemoProducer(ctx context.Context, state *stateManager, hub *eventHub) {
	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()

	users := []string{"demo-user-a", "demo-user-b", "demo-user-c", "demo-user-d", "demo-user-e"}
	userChannels := map[string]string{}
	count := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			channelID := state.randomLeafID()
			if count%3 == 0 {
				s.publishTrigger(triggerPayload{Type: "msg", Ch: channelID}, nil, state, hub)
			} else {
				userID := users[rand.IntN(len(users))]
				s.publishTrigger(triggerPayload{
					Type:         "mov",
					Usr:          userID,
					From:         userChannels[userID],
					To:           channelID,
					Source:       "demo",
					SourceDetail: "server demo producer",
				}, nil, state, hub)
				userChannels[userID] = channelID
			}
			count++
		}
	}
}

func (s *server) consumeTraqStream(ctx context.Context, accessToken string, activeChannelIDs map[string]bool, state *stateManager, hub *eventHub) {
	triggers, errs := s.streamTraqTriggers(ctx, accessToken)
	for {
		select {
		case <-ctx.Done():
			return
		case trigger, ok := <-triggers:
			if !ok {
				triggers = nil
				if errs == nil {
					return
				}
				continue
			}
			s.publishTrigger(trigger, activeChannelIDs, state, hub)
		case err, ok := <-errs:
			if ok && err != nil && ctx.Err() == nil {
				traqLogError("ws stream stopped: %v", err)
				hub.publish(marshalEvent("stream-error", map[string]string{"error": err.Error()}))
			}
			return
		}
	}
}

func (s *server) consumeViewerSnapshots(ctx context.Context, accessToken string, channels []traqChannel, state *stateManager, hub *eventHub) {
	poller := newViewerPoller(channels, s.cfg.viewerChannelsPerTick, state)
	for snapshot := range s.streamViewerSnapshots(ctx, accessToken, poller, hub) {
		hub.publish(marshalEvent("viewers", snapshot))
	}
}
