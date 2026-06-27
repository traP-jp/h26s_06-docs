package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	grandRootID          = "grand_root"
	sessionCookieName    = "traq_session"
	maxConcurrentInits   = 10
	clientEventQueueSize = 64
	recentMessageIDLimit = 100
	maxSyncPayloadDeltas = 100
)

type server struct {
	cfg    config
	client *http.Client

	authMu   sync.Mutex
	states   map[string]time.Time
	sessions map[string]tokenResponse

	liveMu    sync.Mutex
	liveReady bool
	liveData  channelData

	demoState  *stateManager
	demoHub    *eventHub
	liveHub    *eventHub
	initTokens chan struct{}

	demoOnce          sync.Once
	demoCancel        func()
	liveViewersOnce   sync.Once
	liveViewersCancel func()
}

type channel struct {
	ID            string
	Name          string
	ParentID      string
	Children      []string
	IslandID      int
	Depth         int
	Score         float64
	LastSyncScore float64
	LastSyncTime  time.Time
	LastDecayTime time.Time
	LastViewTime  time.Time
}

type userState struct {
	UserID         string
	CurrentChannel string
	LastUpdated    time.Time
}

type stateManager struct {
	mu               sync.RWMutex
	channels         map[string]*channel
	users            map[string]*userState
	seenMessageIDs   map[string]struct{}
	recentMessageIDs []string
	initJSON         []byte
}

type initPayload struct {
	Channels map[string]initChannel `json:"channels"`
}

type initChannel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	ParentID string   `json:"parentId"`
	Children []string `json:"children"`
	IslandID int      `json:"islandId"`
	Depth    int      `json:"depth"`
}

type sseEvent struct {
	Name string
	Data []byte
}

type triggerPayload struct {
	Type         string `json:"type"`
	Ch           string `json:"ch,omitempty"`
	Usr          string `json:"usr,omitempty"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	MessageID    string `json:"-"`
	Source       string `json:"-"`
	SourceDetail string `json:"-"`
}

type syncPayload struct {
	TS     int64              `json:"ts"`
	Deltas map[string]float64 `json:"deltas"`
}

type viewerSnapshotPayload struct {
	TS              int64                  `json:"ts"`
	Total           int                    `json:"total"`
	SampledChannels int                    `json:"sampledChannels"`
	TotalChannels   int                    `json:"totalChannels"`
	Channels        []viewerChannelSummary `json:"channels"`
	Recent          []viewerRow            `json:"recent"`
}

type viewerChannelSummary struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	Count       int    `json:"count"`
	Monitoring  int    `json:"monitoring"`
	Editing     int    `json:"editing"`
	Stale       int    `json:"stale"`
}

type viewerRow struct {
	UserID      string    `json:"userId"`
	ChannelID   string    `json:"channelId"`
	ChannelName string    `json:"channelName"`
	State       string    `json:"state"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type traqChannelList struct {
	Public []traqChannel `json:"public"`
}

type traqChannel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	ParentID *string  `json:"parentId"`
	Children []string `json:"children"`
	Archived bool     `json:"archived"`
}

type traqMessage struct {
	ChannelID string `json:"channelId"`
	User      struct {
		Bot bool `json:"bot"`
	} `json:"user"`
}

type traqViewer struct {
	UserID    string    `json:"userId"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type channelData struct {
	Channels   []traqChannel
	ChannelIDs map[string]bool
	InitJSON   []byte
	State      *stateManager
}

type wsEvent struct {
	Type string          `json:"type"`
	Body json.RawMessage `json:"body"`
}

type wsMessageCreatedBody struct {
	ID string `json:"id"`
}

type wsViewStateChangedBody struct {
	ViewStates []wsViewState `json:"view_states"`
}

type wsViewState struct {
	Key            string `json:"key"`
	ChannelID      string `json:"channelId"`
	ChannelIDSnake string `json:"channel_id"`
	State          string `json:"state"`
}

func (s wsViewState) channelID() string {
	if s.ChannelID != "" {
		return s.ChannelID
	}
	return s.ChannelIDSnake
}
