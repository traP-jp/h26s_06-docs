package main

import (
	"net/http"
	"sync"
	"time"
)

type server struct {
	cfg    config
	client *http.Client

	authMu   sync.Mutex
	states   map[string]time.Time
	sessions map[string]authSession

	userBotMu    sync.Mutex
	userBotCache map[string]bool

	liveMu    sync.Mutex
	liveReady bool
	liveData  channelData

	demoState  *stateManager
	demoHub    *eventHub
	liveHub    *eventHub
	viewerHub  *viewerSignalHub
	initTokens chan struct{}

	demoOnce          sync.Once
	demoCancel        func()
	liveViewersOnce   sync.Once
	liveViewersCancel func()
	authCleanupCancel func()
	demoSyncOnce      sync.Once
	demoSyncCancel    func()
	liveSyncOnce      sync.Once
	liveSyncCancel    func()
}

type sseEvent struct {
	Name string
	Data []byte
}

type triggerPayload struct {
	Type             string  `json:"type"`
	Ch               string  `json:"ch,omitempty"`
	Usr              string  `json:"usr,omitempty"`
	From             string  `json:"from,omitempty"`
	To               string  `json:"to,omitempty"`
	ScoreDelta       float64 `json:"delta"`
	ClearCurrent     bool    `json:"-"`
	MessageID        string  `json:"-"`
	MessageUserID    string  `json:"-"`
	MessageLength    int     `json:"-"`
	HasMessageLength bool    `json:"-"`
	Source           string  `json:"-"`
	SourceDetail     string  `json:"-"`
}

type syncPayload struct {
	TS     int64              `json:"ts"`
	Deltas map[string]float64 `json:"deltas"`
}

type traqChannel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	ParentID *string  `json:"parentId"`
	Children []string `json:"children"`
	Archived bool     `json:"archived"`
}

type channelData struct {
	Channels   []traqChannel
	ChannelIDs map[string]bool
	State      *stateManager
}
