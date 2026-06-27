package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// このファイルは、チャンネルの熱量とユーザー移動状態をメモリ上で管理します。
func newStateManager(vertexCount int) *stateManager {
	now := time.Now()
	channels := map[string]*channel{
		grandRootID: {
			ID:           grandRootID,
			Name:         "Grand Root",
			ParentID:     "",
			Children:     []string{},
			IslandID:     -1,
			Depth:        0,
			LastSyncTime: now,
		},
	}

	if vertexCount < 1 {
		vertexCount = 129
	}

	rootNames := []string{"general", "random", "event", "team", "times", "project", "creative", "tech"}
	numIslands := len(rootNames)
	if vertexCount-1 < numIslands {
		numIslands = vertexCount - 1
	}
	if numIslands < 0 {
		numIslands = 0
	}

	// 1. 島ノード (depth 1) の作成
	for i := 0; i < numIslands; i++ {
		rootID := fmt.Sprintf("island-%d", i+1)
		channels[grandRootID].Children = append(channels[grandRootID].Children, rootID)
		channels[rootID] = &channel{
			ID:           rootID,
			Name:         rootNames[i],
			ParentID:     grandRootID,
			Children:     []string{},
			IslandID:     i,
			Depth:        1,
			LastSyncTime: now,
		}
	}

	// 2. 残りのノードを島に配分して、depth 2〜5 を作成
	remaining := vertexCount - 1 - numIslands
	if remaining > 0 && numIslands > 0 {
		base := remaining / numIslands
		extra := remaining % numIslands

		for i := 0; i < numIslands; i++ {
			rootID := fmt.Sprintf("island-%d", i+1)
			alloc := base
			if i < extra {
				alloc++
			}

			// 収容力を満たす branching factor f を求める
			// 収容容量 capacity = 5f + 10f^2 + 20f^3 + 40f^4
			f := 1
			for {
				capacity := 5*f + 10*f*f + 20*f*f*f + 40*f*f*f*f
				if capacity >= alloc {
					break
				}
				f++
			}

			limit1 := 5 * f
			limit2 := 2 * f
			limit3 := 2 * f
			limit4 := 2 * f

			var levels [6][]string
			levels[1] = []string{rootID}

			childCounter := make(map[string]int)

			for j := 0; j < alloc; j++ {
				var targetDepth int
				var parentID string

				for d := 2; d <= 5; d++ {
					parentLimit := 0
					switch d - 1 {
					case 1:
						parentLimit = limit1
					case 2:
						parentLimit = limit2
					case 3:
						parentLimit = limit3
					case 4:
						parentLimit = limit4
					}

					for _, pID := range levels[d-1] {
						if childCounter[pID] < parentLimit {
							targetDepth = d
							parentID = pID
							break
						}
					}
					if targetDepth != 0 {
						break
					}
				}

				if targetDepth == 0 {
					targetDepth = 5
					parentID = levels[4][len(levels[4])-1]
				}

				childCounter[parentID]++
				idx := childCounter[parentID]

				var nodeID string
				var nodeName string
				switch targetDepth {
				case 2:
					nodeID = fmt.Sprintf("%s-ch-%d", parentID, idx)
					nodeName = fmt.Sprintf("%s/%02d", rootNames[i], idx)
				case 3:
					nodeID = fmt.Sprintf("%s-sub-%d", parentID, idx)
					parentChan := channels[parentID]
					nodeName = fmt.Sprintf("%s/%02d", parentChan.Name, idx)
				case 4:
					nodeID = fmt.Sprintf("%s-leaf-%d", parentID, idx)
					parentChan := channels[parentID]
					nodeName = fmt.Sprintf("%s/%02d", parentChan.Name, idx)
				case 5:
					nodeID = fmt.Sprintf("%s-deep-%d", parentID, idx)
					parentChan := channels[parentID]
					nodeName = fmt.Sprintf("%s/%02d", parentChan.Name, idx)
				}

				channels[parentID].Children = append(channels[parentID].Children, nodeID)
				channels[nodeID] = &channel{
					ID:           nodeID,
					Name:         nodeName,
					ParentID:     parentID,
					Children:     []string{},
					IslandID:     i,
					Depth:        targetDepth,
					LastSyncTime: now,
				}
				levels[targetDepth] = append(levels[targetDepth], nodeID)
			}
		}
	}

	return &stateManager{
		channels: channels,
		users:    map[string]*userState{},
	}
}

func (sm *stateManager) initJSON() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	payload := initPayload{Channels: make(map[string]initChannel, len(sm.channels))}
	for id, ch := range sm.channels {
		payload.Channels[id] = initChannel{
			ID:       ch.ID,
			Name:     ch.Name,
			ParentID: ch.ParentID,
			Children: append([]string{}, ch.Children...),
			IslandID: ch.IslandID,
			Depth:    ch.Depth,
		}
	}
	return json.Marshal(payload)
}

func (sm *stateManager) applyTrigger(trigger triggerPayload) (triggerPayload, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch trigger.Type {
	case "msg":
		sm.addScoreLocked(trigger.Ch, 46)
	case "mov":
		if trigger.Usr != "" && trigger.To != "" {
			user, ok := sm.users[trigger.Usr]
			if !ok {
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
	}
	return trigger, true
}

func (sm *stateManager) addScoreLocked(channelID string, amount float64) {
	for depth := 0; channelID != ""; depth++ {
		ch, ok := sm.channels[channelID]
		if !ok {
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
		if math.Abs(ch.Score-ch.LastSyncScore) >= 0.35 || (ch.Score > 0.1 && elapsed >= 8) {
			deltas[ch.ID] = math.Round(ch.Score*10) / 10
			ch.LastSyncScore = ch.Score
			ch.LastSyncTime = now
		}
	}
	return syncPayload{TS: now.Unix(), Deltas: deltas}
}

func (sm *stateManager) randomChannelID() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	candidates := make([]string, 0, len(sm.channels))
	for id, ch := range sm.channels {
		if id != grandRootID && len(ch.Children) == 0 {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return grandRootID
	}
	return candidates[rand.Intn(len(candidates))]
}
