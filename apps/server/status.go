package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type statusRequest struct {
	Channel   *string `json:"channel"`
	ChannelID *string `json:"channelId"`
}

func (r statusRequest) channelID() (string, bool) {
	if r.Channel != nil {
		return *r.Channel, true
	}
	if r.ChannelID != nil {
		return *r.ChannelID, true
	}
	return "", false
}

func (s *server) handleStatus(c echo.Context) error {
	sessionID, session, ok := s.session(c.Request())
	if !ok {
		return echoHTTPError(c, "not authenticated", http.StatusUnauthorized)
	}

	var req statusRequest
	if err := c.Bind(&req); err != nil {
		return echoHTTPError(c, "invalid status payload", http.StatusBadRequest)
	}
	channelID, ok := req.channelID()
	if !ok {
		return echoHTTPError(c, "channel is required", http.StatusBadRequest)
	}

	if s.cfg.traqBotAccessToken == "" {
		return echoHTTPError(c, "TRAQ_BOT_ACCESS_TOKEN is not configured", http.StatusServiceUnavailable)
	}

	data, err := s.ensureLiveChannelData(c.Request().Context(), s.cfg.traqBotAccessToken)
	if err != nil {
		traqLogError("failed to load channel data for status update: %v", err)
		return echoHTTPError(c, "failed to load channel data", http.StatusBadGateway)
	}
	if channelID != "" && !data.ChannelIDs[channelID] {
		return echoHTTPError(c, "unknown channel", http.StatusBadRequest)
	}

	userID, err := s.ensureSessionTraqUserID(c.Request().Context(), sessionID, session)
	if err != nil {
		traqLogError("failed to fetch traQ user info for status update: %v", err)
		return echoHTTPError(c, "failed to fetch user info", http.StatusBadGateway)
	}
	if channelID == "" {
		data.State.clearUserStatus(userID)
	} else if !data.State.setUserStatus(userID, channelID) {
		return echoHTTPError(c, "unknown channel", http.StatusBadRequest)
	}
	if channelID != "" && s.viewerHub != nil {
		s.viewerHub.publish(viewerSignal{ChannelID: channelID})
	}

	return c.NoContent(http.StatusNoContent)
}
