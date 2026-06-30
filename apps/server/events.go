package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

const maxConcurrentInits = 10

func (s *server) handleEvents(c echo.Context) error {
	r := c.Request()
	w := c.Response().Writer
	demo := c.QueryParam("demo") == "1"
	var sessionID string
	var session authSession

	if !demo {
		var ok bool
		sessionID, session, ok = s.session(r)
		if !ok {
			return echoHTTPError(c, "not authenticated", http.StatusUnauthorized)
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return echoHTTPError(c, "streaming unsupported", http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	streamState := s.demoState
	streamHub := s.demoHub
	var liveChannelIDs map[string]bool
	var currentUserID string

	if !demo {
		data, err := s.ensureLiveChannelData(r.Context(), session.Token.AccessToken)
		if err != nil {
			writeSSE(w, marshalEvent("stream-error", map[string]string{"error": err.Error()}))
			flusher.Flush()
			return nil
		}
		streamState = data.State
		streamHub = s.liveHub
		liveChannelIDs = data.ChannelIDs
		currentUserID, err = s.ensureSessionTraqUserID(r.Context(), sessionID, session)
		if err != nil {
			writeSSE(w, marshalEvent("stream-error", map[string]string{"error": err.Error()}))
			flusher.Flush()
			return nil
		}
		s.startLiveSyncProducer(streamState)
	}

	initPayload := streamState.initPayloadBytes()

	select {
	case s.initTokens <- struct{}{}:
		writeSSE(w, sseEvent{Name: "init", Data: initPayload})
		flusher.Flush()
		<-s.initTokens
	case <-r.Context().Done():
		return nil
	}

	events := streamHub.subscribe()
	defer streamHub.unsubscribe(events)

	writeSSE(w, marshalEvent("status", map[string]string{"status": streamStatus(demo)}))
	flusher.Flush()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	var viewerEvents <-chan sseEvent
	if demo {
		s.startDemoProducer()
		s.startDemoSyncProducer()
	} else {
		viewerEvents = s.streamCurrentViewerEvents(ctx, currentUserID, streamState, liveChannelIDs)
		go s.consumeTraqStream(ctx, session.Token.AccessToken, liveChannelIDs, streamState, streamHub)
	}

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			writeSSE(w, event)
			flusher.Flush()
		case event, ok := <-viewerEvents:
			if !ok {
				viewerEvents = nil
				continue
			}
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (s *server) runSyncProducer(ctx context.Context, state *stateManager, hub *eventHub) {
	ticker := time.NewTicker(s.cfg.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			publishSyncPayload(state, hub)
		}
	}
}

func (s *server) runLiveSyncProducer(ctx context.Context, state *stateManager, hub *eventHub) {
	ticker := time.NewTicker(s.cfg.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			publishSyncPayload(state, hub)
			s.persistChannelScores(state)
		}
	}
}

func publishSyncPayload(state *stateManager, hub *eventHub) {
	payload := state.syncPayload()
	if len(payload.Deltas) > 0 {
		hub.publish(marshalEvent("sync", payload))
	}
}

func (s *server) persistChannelScores(state *stateManager) {
	if s.store == nil {
		return
	}
	records := state.scoreRecords()
	if len(records) == 0 {
		return
	}
	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.store.SaveChannelScores(saveCtx, records); err != nil {
		traqLogWarn("persist channel scores failed: %v", err)
	}
}

func streamStatus(demo bool) string {
	if demo {
		return "demo connected"
	}
	return "traQ connected"
}

func (s *server) publishTrigger(trigger triggerPayload, activeChannelIDs map[string]bool, state *stateManager, hub *eventHub) (triggerPayload, bool) {
	if activeChannelIDs != nil && !triggerInActiveChannels(trigger, activeChannelIDs) {
		if trigger.Type == "mov" {
			debugMov(trigger, "", "", "skipped", "destination channel is not in active channel set", 0)
		}
		return trigger, false
	}
	applied, changed := state.applyTrigger(trigger)
	if !changed {
		return applied, false
	}
	hub.publish(marshalEvent("trigger", applied))
	return applied, true
}

func triggerInActiveChannels(trigger triggerPayload, active map[string]bool) bool {
	switch trigger.Type {
	case "msg":
		return active[trigger.Ch]
	case "mov":
		if trigger.ClearCurrent {
			return active[trigger.From]
		}
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
	events, errs := s.streamTraqEvents(ctx, accessToken, state)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				events = nil
				if errs == nil {
					return
				}
				continue
			}
			for _, trigger := range event.Triggers {
				s.publishTrigger(trigger, activeChannelIDs, state, hub)
			}
			for _, update := range event.ViewerUpdates {
				publishViewerUpdate(update, activeChannelIDs, hub)
			}
		case err, ok := <-errs:
			if ok && err != nil && ctx.Err() == nil {
				traqLogError("ws stream stopped: %v", err)
				hub.publish(marshalEvent("stream-error", map[string]string{"error": err.Error()}))
			}
			return
		}
	}
}

func publishViewerUpdate(update viewerUpdate, activeChannelIDs map[string]bool, hub *eventHub) bool {
	sampledChannelIDs := make(map[string]bool, len(update.SampledChannelIDs))
	for channelID := range update.SampledChannelIDs {
		if activeChannelIDs == nil || activeChannelIDs[channelID] {
			sampledChannelIDs[channelID] = true
		}
	}

	rows := make([]viewerRow, 0, len(update.Rows))
	for _, row := range update.Rows {
		if activeChannelIDs != nil && !activeChannelIDs[row.ChannelID] {
			continue
		}
		rows = append(rows, row)
		if row.ChannelID != "" {
			sampledChannelIDs[row.ChannelID] = true
		}
	}
	if len(sampledChannelIDs) == 0 {
		return false
	}

	totalChannelCount := len(activeChannelIDs)
	if totalChannelCount == 0 {
		totalChannelCount = len(sampledChannelIDs)
	}
	hub.publish(marshalEvent("viewers", viewerSnapshotFromRows(rows, len(sampledChannelIDs), totalChannelCount, time.Now())))
	return true
}

func (s *server) consumeViewerSnapshots(ctx context.Context, accessToken string, channels []traqChannel, state *stateManager, hub *eventHub) {
	poller := newViewerPoller(channels, s.cfg.viewerChannelsPerTick, state)
	for snapshot := range s.streamViewerSnapshots(ctx, accessToken, poller, hub) {
		hub.publish(marshalEvent("viewers", snapshot))
	}
}
