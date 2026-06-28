package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	grandRootID          = "grand_root"
	recentMessageIDLimit = 100
	maxSyncPayloadDeltas = 100

	messageScoreAmount         = 1.0
	messageScoreReferenceChars = 20
	movementScoreAmount        = 0.025
	ancestorScoreFactor        = 0.45
	scoreDecayTimeScale        = 300.0
	syncDeltaWeightScale       = 10.0
	viewerScoreWeight          = 0.46
)

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
	UserID            string
	CurrentChannel    string
	LastViewedChannel string
	LastUpdated       time.Time
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
	Score    float64  `json:"score"`
}

func newDemoStateManager() (*stateManager, error) {
	now := time.Now()
	channels := map[string]*channel{
		grandRootID: {
			ID:           grandRootID,
			Name:         "traQ",
			ParentID:     "",
			IslandID:     -1,
			Depth:        0,
			LastSyncTime: now,
		},
	}

	roots := []string{"general", "random", "event", "team", "times", "project", "creative", "tech", "music"}
	for islandID, name := range roots {
		rootID := fmt.Sprintf("demo-root-%02d", islandID+1)
		channels[grandRootID].Children = append(channels[grandRootID].Children, rootID)
		channels[rootID] = &channel{
			ID:           rootID,
			Name:         name,
			ParentID:     grandRootID,
			IslandID:     islandID,
			Depth:        1,
			LastSyncTime: now,
		}
		for i := 0; i < 8; i++ {
			childID := fmt.Sprintf("%s-%02d", rootID, i+1)
			channels[rootID].Children = append(channels[rootID].Children, childID)
			channels[childID] = &channel{
				ID:           childID,
				Name:         fmt.Sprintf("%s/%02d", name, i+1),
				ParentID:     rootID,
				IslandID:     islandID,
				Depth:        2,
				LastSyncTime: now,
			}
			for j := 0; j < 3; j++ {
				leafID := fmt.Sprintf("%s-%02d", childID, j+1)
				channels[childID].Children = append(channels[childID].Children, leafID)
				channels[leafID] = &channel{
					ID:           leafID,
					Name:         fmt.Sprintf("%s/%02d/%02d", name, i+1, j+1),
					ParentID:     childID,
					IslandID:     islandID,
					Depth:        3,
					LastSyncTime: now,
				}
			}
		}
	}

	prepareChannelTimes(channels, now)
	sm := &stateManager{channels: channels, users: map[string]*userState{}, seenMessageIDs: map[string]struct{}{}}
	if err := sm.rebuildInitJSONLocked(); err != nil {
		return nil, err
	}
	return sm, nil
}

func newStateManagerFromTraq(channels []traqChannel) (*stateManager, error) {
	now := time.Now()
	nodes := map[string]*channel{
		grandRootID: {
			ID:           grandRootID,
			Name:         "traQ",
			ParentID:     "",
			IslandID:     -1,
			Depth:        0,
			LastSyncTime: now,
		},
	}

	included := make(map[string]traqChannel, len(channels))
	for _, ch := range channels {
		if ch.ID != "" && !ch.Archived {
			included[ch.ID] = ch
		}
	}
	for _, ch := range channels {
		if _, ok := included[ch.ID]; !ok {
			continue
		}
		parentID := grandRootID
		if ch.ParentID != nil {
			if _, ok := included[*ch.ParentID]; ok {
				parentID = *ch.ParentID
			}
		}
		nodes[ch.ID] = &channel{
			ID:           ch.ID,
			Name:         ch.Name,
			ParentID:     parentID,
			LastSyncTime: now,
		}
	}
	for id, ch := range nodes {
		if id == grandRootID {
			continue
		}
		parent, ok := nodes[ch.ParentID]
		if !ok {
			ch.ParentID = grandRootID
			parent = nodes[grandRootID]
		}
		parent.Children = append(parent.Children, id)
	}
	for islandID, rootID := range nodes[grandRootID].Children {
		assignLayout(nodes, rootID, islandID, 1)
	}

	prepareChannelTimes(nodes, now)
	sm := &stateManager{channels: nodes, users: map[string]*userState{}, seenMessageIDs: map[string]struct{}{}}
	if err := sm.rebuildInitJSONLocked(); err != nil {
		return nil, err
	}
	return sm, nil
}

func assignLayout(channels map[string]*channel, id string, islandID int, depth int) {
	ch, ok := channels[id]
	if !ok {
		return
	}
	ch.IslandID = islandID
	ch.Depth = depth
	for _, childID := range ch.Children {
		assignLayout(channels, childID, islandID, depth+1)
	}
}

func prepareChannelTimes(channels map[string]*channel, now time.Time) {
	for _, ch := range channels {
		if ch.LastSyncTime.IsZero() {
			ch.LastSyncTime = now
		}
		if ch.LastDecayTime.IsZero() {
			ch.LastDecayTime = now
		}
	}
}

func (sm *stateManager) initPayloadBytes() []byte {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.decayScoresLocked(time.Now())
	data, err := sm.marshalInitPayloadLocked()
	if err != nil {
		return append([]byte(nil), sm.initJSON...)
	}
	sm.initJSON = data
	return append([]byte(nil), data...)
}

func (sm *stateManager) setUserStatus(userID string, channelID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if userID == "" || channelID == "" || sm.channels[channelID] == nil {
		return false
	}
	user := sm.users[userID]
	if user == nil {
		user = &userState{UserID: userID}
		sm.users[userID] = user
	}
	user.CurrentChannel = channelID
	user.LastViewedChannel = channelID
	user.LastUpdated = time.Now()
	return true
}

func (sm *stateManager) clearUserStatus(userID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if userID == "" {
		return false
	}
	user := sm.users[userID]
	if user == nil {
		return false
	}
	user.CurrentChannel = ""
	user.LastUpdated = time.Now()
	return true
}

func (sm *stateManager) currentChannel(userID string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	user := sm.users[userID]
	if user == nil {
		return ""
	}
	return user.CurrentChannel
}

func (sm *stateManager) applyTrigger(trigger triggerPayload) (triggerPayload, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch trigger.Type {
	case "msg":
		if trigger.Ch == "" || sm.channels[trigger.Ch] == nil {
			return trigger, false
		}
		if trigger.MessageID != "" {
			if _, ok := sm.seenMessageIDs[trigger.MessageID]; ok {
				return trigger, false
			}
			sm.rememberMessageIDLocked(trigger.MessageID)
		}
		score := messageScoreDelta(trigger)
		sm.addScoreLocked(trigger.Ch, score)
		trigger.ScoreDelta = score
		return trigger, true
	case "mov":
		if trigger.ClearCurrent {
			if trigger.Usr == "" || trigger.From == "" {
				return trigger, false
			}
			user := sm.users[trigger.Usr]
			if user == nil || user.CurrentChannel != trigger.From {
				return trigger, false
			}
			user.CurrentChannel = ""
			user.LastUpdated = time.Now()
			return trigger, false
		}
		if trigger.To == "" || sm.channels[trigger.To] == nil {
			debugMov(trigger, "", "", "skipped", "destination channel is empty or unknown", 0)
			return trigger, false
		}
		toName := sm.channels[trigger.To].Name
		fromName := ""
		if trigger.Usr != "" {
			user := sm.users[trigger.Usr]
			if user == nil {
				user = &userState{UserID: trigger.Usr}
				sm.users[trigger.Usr] = user
			}
			if trigger.From == "" {
				trigger.From = user.CurrentChannel
				if trigger.From == "" {
					trigger.From = user.LastViewedChannel
				}
			}
			if trigger.From == trigger.To || user.LastViewedChannel == trigger.To {
				debugMov(trigger, toName, toName, "skipped", "user is already in the destination channel", 0)
				return trigger, false
			}
			user.CurrentChannel = trigger.To
			user.LastViewedChannel = trigger.To
			user.LastUpdated = time.Now()
		}
		if from := sm.channels[trigger.From]; from != nil {
			fromName = from.Name
		}
		score := movementScoreAmount
		sm.addScoreLocked(trigger.To, score)
		trigger.ScoreDelta = score
		debugMov(trigger, fromName, toName, "applied", "user moved to a different channel; destination channel and ancestors receive movement score", score)
		return trigger, true
	default:
		return trigger, false
	}
}

func messageScoreDelta(trigger triggerPayload) float64 {
	if !trigger.HasMessageLength {
		return messageScoreAmount
	}
	if trigger.MessageLength <= 0 {
		return 0
	}
	return messageScoreAmount *
		math.Log1p(float64(trigger.MessageLength)) /
		math.Log1p(float64(messageScoreReferenceChars))
}

func (sm *stateManager) rememberMessageIDLocked(messageID string) {
	sm.seenMessageIDs[messageID] = struct{}{}
	sm.recentMessageIDs = append(sm.recentMessageIDs, messageID)
	if len(sm.recentMessageIDs) <= recentMessageIDLimit {
		return
	}
	evicted := sm.recentMessageIDs[0]
	copy(sm.recentMessageIDs, sm.recentMessageIDs[1:])
	sm.recentMessageIDs = sm.recentMessageIDs[:len(sm.recentMessageIDs)-1]
	delete(sm.seenMessageIDs, evicted)
}

func (sm *stateManager) addScoreLocked(channelID string, amount float64) {
	for depth := 0; channelID != ""; depth++ {
		ch := sm.channels[channelID]
		if ch == nil {
			return
		}
		ch.Score += amount * math.Pow(ancestorScoreFactor, float64(depth))
		channelID = ch.ParentID
	}
}

func (sm *stateManager) syncPayload() syncPayload {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	sm.decayScoresLocked(now)

	weighted := make([]weightedChannel, 0, len(sm.channels))
	for _, ch := range sm.channels {
		elapsed := now.Sub(ch.LastSyncTime).Seconds()
		deltaScore := math.Abs(ch.Score - ch.LastSyncScore)
		weight := syncPayloadWeight(deltaScore, elapsed)
		weighted = append(weighted, weightedChannel{id: ch.ID, rawWeight: weight})
	}

	deltas := make(map[string]float64)
	for _, selected := range selectWeightedChannels(weighted, maxSyncPayloadDeltas) {
		ch := sm.channels[selected.id]
		if ch == nil {
			continue
		}
		deltas[ch.ID] = roundedScore(ch.Score)
		ch.LastSyncScore = ch.Score
		ch.LastSyncTime = now
	}
	return syncPayload{TS: now.Unix(), Deltas: deltas}
}

func (sm *stateManager) decayScoresLocked(now time.Time) {
	for _, ch := range sm.channels {
		decayElapsed := now.Sub(ch.LastDecayTime).Seconds()
		if decayElapsed > 0 {
			ch.Score *= math.Exp(-decayElapsed / scoreDecayTimeScale)
		}
		ch.LastDecayTime = now
	}
}

func roundedScore(score float64) float64 {
	return math.Round(score*1000) / 1000
}

func syncPayloadWeight(deltaScore float64, elapsedSeconds float64) float64 {
	return syncDeltaWeightScale*deltaScore + 0.002*elapsedSeconds
}

func (sm *stateManager) sampleViewerChannels(candidates []traqChannel, maxChannels int) []traqChannel {
	if maxChannels <= 0 || len(candidates) == 0 {
		return nil
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	weighted := make([]weightedChannel, 0, len(candidates))
	for _, candidate := range candidates {
		ch := sm.channels[candidate.ID]
		if ch == nil {
			continue
		}
		probability := 1.0
		if !ch.LastViewTime.IsZero() {
			elapsed := now.Sub(ch.LastViewTime).Seconds()
			probability = viewerPollWeight(ch.Score, elapsed)
		}
		weighted = append(weighted, weightedChannel{id: candidate.ID, channel: candidate, rawWeight: probability})
	}

	selected := selectWeightedChannels(weighted, maxChannels)
	channels := make([]traqChannel, 0, len(selected))
	for _, selectedChannel := range selected {
		if ch := sm.channels[selectedChannel.id]; ch != nil {
			ch.LastViewTime = now
		}
		channels = append(channels, selectedChannel.channel)
	}
	return channels
}

func viewerPollWeight(score float64, elapsedSeconds float64) float64 {
	return viewerScoreWeight*score + 0.001*elapsedSeconds
}

func (sm *stateManager) randomLeafID() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	leaves := make([]string, 0, len(sm.channels))
	for id, ch := range sm.channels {
		if id != grandRootID && len(ch.Children) == 0 {
			leaves = append(leaves, id)
		}
	}
	if len(leaves) == 0 {
		return grandRootID
	}
	return leaves[rand.IntN(len(leaves))]
}

func (sm *stateManager) rebuildInitJSONLocked() error {
	data, err := sm.marshalInitPayloadLocked()
	if err != nil {
		return err
	}
	sm.initJSON = data
	return nil
}

func (sm *stateManager) marshalInitPayloadLocked() ([]byte, error) {
	payload := initPayload{Channels: make(map[string]initChannel, len(sm.channels))}
	for id, ch := range sm.channels {
		children := ch.Children
		if children == nil {
			children = []string{}
		}
		payload.Channels[id] = initChannel{
			ID:       ch.ID,
			Name:     ch.Name,
			ParentID: ch.ParentID,
			Children: children,
			IslandID: ch.IslandID,
			Depth:    ch.Depth,
			Score:    roundedScore(ch.Score),
		}
	}
	return json.Marshal(payload)
}
