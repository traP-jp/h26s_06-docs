package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type statusRequest struct {
	Channel   string `json:"channel"`
	ChannelID string `json:"channelId"`
}

func (r statusRequest) channelID() string {
	if r.Channel != "" {
		return r.Channel
	}
	return r.ChannelID
}

func (s *server) handleStatus(c echo.Context) error {
	token, ok := s.sessionToken(c.Request())
	if !ok {
		return echoHTTPError(c, "not authenticated", http.StatusUnauthorized)
	}

	var req statusRequest
	if err := c.Bind(&req); err != nil {
		return echoHTTPError(c, "invalid status payload", http.StatusBadRequest)
	}
	channelID := req.channelID()
	if channelID == "" {
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
	if !data.ChannelIDs[channelID] {
		return echoHTTPError(c, "unknown channel", http.StatusBadRequest)
	}

	me, err := s.fetchTraqMe(c.Request().Context(), token.AccessToken)
	if err != nil {
		traqLogError("failed to fetch traQ user info for status update: %v", err)
		return echoHTTPError(c, "failed to fetch user info", http.StatusBadGateway)
	}
	userID := me.ID
	if userID == "" {
		userID = me.Name
	}
	if !data.State.setUserStatus(userID, channelID) {
		return echoHTTPError(c, "unknown channel", http.StatusBadRequest)
	}

	return c.NoContent(http.StatusNoContent)
}
