package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

func newDemoStateManager() (*stateManager, error) {
	now := time.Now()
	channels := map[string]*channel{
		grandRootID: {
			ID:           grandRootID,
			Name:         "Grand Root",
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

	sm := &stateManager{channels: channels, users: map[string]*userState{}}
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
			Name:         "Grand Root",
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

	sm := &stateManager{channels: nodes, users: map[string]*userState{}}
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

func (sm *stateManager) initPayloadBytes() []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return append([]byte(nil), sm.initJSON...)
}

func (sm *stateManager) applyTrigger(trigger triggerPayload) (triggerPayload, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch trigger.Type {
	case "msg":
		if trigger.Ch == "" || sm.channels[trigger.Ch] == nil {
			return trigger, false
		}
		sm.addScoreLocked(trigger.Ch, 46)
		return trigger, true
	case "mov":
		if trigger.To == "" || sm.channels[trigger.To] == nil {
			return trigger, false
		}
		if trigger.Usr != "" {
			user := sm.users[trigger.Usr]
			if user == nil {
				user = &userState{UserID: trigger.Usr}
				sm.users[trigger.Usr] = user
			}
			if trigger.From == "" {
				trigger.From = user.CurrentChannel
			}
			if trigger.From == trigger.To {
				return trigger, false
			}
			user.CurrentChannel = trigger.To
			user.LastUpdated = time.Now()
		}
		sm.addScoreLocked(trigger.To, 11)
		return trigger, true
	default:
		return trigger, false
	}
}

func (sm *stateManager) addScoreLocked(channelID string, amount float64) {
	for depth := 0; channelID != ""; depth++ {
		ch := sm.channels[channelID]
		if ch == nil {
			return
		}
		ch.Score = math.Min(100, ch.Score+amount*math.Pow(0.45, float64(depth)))
		channelID = ch.ParentID
	}
}

func (sm *stateManager) syncPayload() syncPayload {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	deltas := make(map[string]float64)
	for _, ch := range sm.channels {
		elapsed := now.Sub(ch.LastSyncTime).Seconds()
		ch.Score *= math.Exp(-elapsed / 24)

		deltaScore := math.Abs(ch.Score - ch.LastSyncScore)
		probability := math.Min(1, 0.22*deltaScore+0.002*elapsed)
		if rand.Float64() < probability || (ch.Score > 0.1 && elapsed >= 30) {
			deltas[ch.ID] = math.Round(ch.Score*10) / 10
			ch.LastSyncScore = ch.Score
			ch.LastSyncTime = now
		}
	}
	return syncPayload{TS: now.Unix(), Deltas: deltas}
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
	payload := initPayload{Channels: make(map[string]initChannel, len(sm.channels))}
	for id, ch := range sm.channels {
		payload.Channels[id] = initChannel{
			ID:       ch.ID,
			Name:     ch.Name,
			ParentID: ch.ParentID,
			Children: append([]string(nil), ch.Children...),
			IslandID: ch.IslandID,
			Depth:    ch.Depth,
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sm.initJSON = data
	return nil
}
