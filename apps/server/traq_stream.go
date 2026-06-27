package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func (s *server) streamTraqTriggers(ctx context.Context, accessToken string) (<-chan triggerPayload, <-chan error) {
	out := make(chan triggerPayload)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errs)

		wsURL := strings.Replace(s.cfg.traqBaseURL+"/api/v3/ws", "https://", "wss://", 1)
		wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

		header := http.Header{}
		header.Set("Authorization", "Bearer "+accessToken)
		traqLogWS("dial %s", wsURL)
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
		if err != nil {
			errs <- fmt.Errorf("websocket dial failed: %w", err)
			return
		}
		defer conn.Close()
		traqLogOK("ws connected %s", wsURL)

		if err := conn.WriteMessage(websocket.TextMessage, []byte("timeline_streaming:on")); err != nil {
			errs <- fmt.Errorf("websocket command failed: %w", err)
			return
		}
		traqLogWS("sent command timeline_streaming:on")
		go func() {
			<-ctx.Done()
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			_ = conn.Close()
		}()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				if ctx.Err() == nil {
					errs <- fmt.Errorf("websocket read failed: %w", err)
				}
				return
			}
			triggers, err := s.parseTraqEvent(ctx, accessToken, payload)
			if err != nil {
				traqLogError("skip ws event: %v", err)
				continue
			}
			for _, trigger := range triggers {
				select {
				case out <- trigger:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, errs
}

func (s *server) parseTraqEvent(ctx context.Context, accessToken string, payload []byte) ([]triggerPayload, error) {
	var event wsEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	eventType := strings.ToUpper(event.Type)
	traqLogWS("received type=%s bodyBytes=%d", eventType, len(event.Body))

	switch eventType {
	case "MESSAGE_CREATED":
		var body wsMessageCreatedBody
		if err := json.Unmarshal(event.Body, &body); err != nil {
			return nil, err
		}
		if body.ID == "" {
			traqLogWarn("MESSAGE_CREATED skipped: empty message id")
			return nil, nil
		}
		traqLogWS("MESSAGE_CREATED messageID=%s", body.ID)
		channelID, isBot, err := s.fetchMessageInfo(ctx, s.cfg.traqBotAccessToken, body.ID)
		if err != nil || isBot || channelID == "" {
			if err == nil && isBot {
				traqLogWarn("MESSAGE_CREATED skipped: bot messageID=%s channelID=%s", body.ID, channelID)
			}
			if err == nil && !isBot && channelID == "" {
				traqLogWarn("MESSAGE_CREATED skipped: empty channel messageID=%s", body.ID)
			}
			return nil, err
		}
		traqLogOK("MESSAGE_CREATED accepted messageID=%s channelID=%s", body.ID, channelID)
		return []triggerPayload{{
			Type:         "msg",
			Ch:           channelID,
			MessageID:    body.ID,
			Source:       "ws",
			SourceDetail: "traQ /api/v3/ws timeline_streaming:on MESSAGE_CREATED",
		}}, nil
	case "USER_VIEWSTATE_CHANGED":
		var body wsViewStateChangedBody
		if err := json.Unmarshal(event.Body, &body); err != nil {
			return nil, err
		}
		triggers := make([]triggerPayload, 0, len(body.ViewStates))
		for _, view := range body.ViewStates {
			channelID := view.channelID()
			if view.Key == "" || channelID == "" {
				continue
			}
			trigger := triggerPayload{
				Type:         "mov",
				Usr:          hashViewerKey(view.Key),
				Source:       "ws",
				SourceDetail: "traQ /api/v3/ws timeline_streaming:on USER_VIEWSTATE_CHANGED",
			}
			if view.State == "none" {
				trigger.From = channelID
				trigger.ClearCurrent = true
				triggers = append(triggers, trigger)
				continue
			}
			trigger.To = channelID
			triggers = append(triggers, trigger)
		}
		traqLogWS("USER_VIEWSTATE_CHANGED viewStates=%d triggers=%d", len(body.ViewStates), len(triggers))
		return triggers, nil
	default:
		return nil, nil
	}
}

func (s *server) fetchMessageInfo(ctx context.Context, accessToken string, messageID string) (string, bool, error) {
	traqLogAPI("GET /api/v3/messages/%s", messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/messages/"+url.PathEscape(messageID), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		traqLogError("GET /api/v3/messages/%s failed: %v", messageID, err)
		return "", false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		traqLogWarn("GET /api/v3/messages/%s -> %s", messageID, resp.Status)
		return "", false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		traqLogError("GET /api/v3/messages/%s -> %s", messageID, resp.Status)
		return "", false, fmt.Errorf("message endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var message traqMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return "", false, err
	}
	if message.UserID == "" {
		return "", false, fmt.Errorf("message endpoint returned empty userId for %s", messageID)
	}

	if isBot, ok := s.cachedUserIsBot(message.UserID); ok {
		traqLogOK("GET /api/v3/messages/%s -> %s channelID=%s userID=%s bot=%t botCache=hit", messageID, resp.Status, message.ChannelID, message.UserID, isBot)
		return message.ChannelID, isBot, nil
	}

	isBot, err := s.fetchUserIsBot(ctx, accessToken, message.UserID)
	if err != nil {
		return "", false, err
	}
	s.storeUserIsBot(message.UserID, isBot)
	traqLogOK("GET /api/v3/messages/%s -> %s channelID=%s userID=%s bot=%t botCache=miss", messageID, resp.Status, message.ChannelID, message.UserID, isBot)
	return message.ChannelID, isBot, nil
}

func (s *server) cachedUserIsBot(userID string) (bool, bool) {
	s.userBotMu.Lock()
	defer s.userBotMu.Unlock()

	isBot, ok := s.userBotCache[userID]
	if !ok {
		return false, false
	}
	return isBot, true
}

func (s *server) storeUserIsBot(userID string, isBot bool) {
	s.userBotMu.Lock()
	defer s.userBotMu.Unlock()

	if s.userBotCache == nil {
		s.userBotCache = map[string]bool{}
	}
	if _, ok := s.userBotCache[userID]; !ok && len(s.userBotCache) >= userBotCacheLimit {
		s.evictRandomUserBotLocked()
	}
	s.userBotCache[userID] = isBot
}

func (s *server) evictRandomUserBotLocked() {
	if len(s.userBotCache) == 0 {
		return
	}
	target := rand.Intn(len(s.userBotCache))
	i := 0
	for userID := range s.userBotCache {
		if i == target {
			delete(s.userBotCache, userID)
			return
		}
		i++
	}
}

func (s *server) fetchUserIsBot(ctx context.Context, accessToken string, userID string) (bool, error) {
	traqLogAPI("GET /api/v3/users/%s", userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/users/"+url.PathEscape(userID), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		traqLogError("GET /api/v3/users/%s failed: %v", userID, err)
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		traqLogError("GET /api/v3/users/%s -> %s", userID, resp.Status)
		return false, fmt.Errorf("user endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var user traqUser
	if err := json.Unmarshal(body, &user); err != nil {
		return false, err
	}
	traqLogOK("GET /api/v3/users/%s -> %s bot=%t", userID, resp.Status, user.Bot)
	return user.Bot, nil
}

func hashViewerKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "session_" + hex.EncodeToString(sum[:])[:12]
}
