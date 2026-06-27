package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
		if err != nil {
			errs <- fmt.Errorf("websocket dial failed: %w", err)
			return
		}
		defer conn.Close()

		if err := conn.WriteMessage(websocket.TextMessage, []byte("timeline_streaming:on")); err != nil {
			errs <- fmt.Errorf("websocket command failed: %w", err)
			return
		}
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
				log.Printf("skipping traQ event: %v", err)
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

	switch strings.ToUpper(event.Type) {
	case "MESSAGE_CREATED":
		var body wsMessageCreatedBody
		if err := json.Unmarshal(event.Body, &body); err != nil {
			return nil, err
		}
		if body.ID == "" {
			return nil, nil
		}
		channelID, isBot, err := s.fetchMessageInfo(ctx, accessToken, body.ID)
		if err != nil || isBot || channelID == "" {
			return nil, err
		}
		return []triggerPayload{{Type: "msg", Ch: channelID}}, nil
	case "USER_VIEWSTATE_CHANGED":
		var body wsViewStateChangedBody
		if err := json.Unmarshal(event.Body, &body); err != nil {
			return nil, err
		}
		triggers := make([]triggerPayload, 0, len(body.ViewStates))
		for _, view := range body.ViewStates {
			channelID := view.channelID()
			if view.Key == "" || channelID == "" || view.State == "none" {
				continue
			}
			triggers = append(triggers, triggerPayload{
				Type: "mov",
				Usr:  hashViewerKey(view.Key),
				To:   channelID,
			})
		}
		return triggers, nil
	default:
		return nil, nil
	}
}

func (s *server) fetchMessageInfo(ctx context.Context, accessToken string, messageID string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.traqBaseURL+"/api/v3/messages/"+url.PathEscape(messageID), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("message endpoint returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var message traqMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return "", false, err
	}
	return message.ChannelID, message.User.Bot, nil
}

func hashViewerKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "session_" + hex.EncodeToString(sum[:])[:12]
}
