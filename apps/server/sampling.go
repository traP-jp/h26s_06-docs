package main

import "math/rand/v2"

type weightedChannel struct {
	id               string
	channel          traqChannel
	rawWeight        float64
	normalizedWeight float64
}

func selectWeightedChannels(candidates []weightedChannel, maxChannels int) []weightedChannel {
	if maxChannels <= 0 || len(candidates) == 0 {
		return nil
	}
	candidates = normalizeWeightedChannels(candidates)
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) <= maxChannels {
		return candidates
	}
	selected := make([]weightedChannel, 0, maxChannels)
	for len(selected) < maxChannels && len(candidates) > 0 {
		total := 0.0
		for _, c := range candidates {
			total += c.normalizedWeight
		}
		pick := rand.Float64() * total
		index := 0
		for i, c := range candidates {
			pick -= c.normalizedWeight
			if pick <= 0 {
				index = i
				break
			}
		}
		selected = append(selected, candidates[index])
		candidates = append(candidates[:index], candidates[index+1:]...)
	}
	return selected
}

func normalizeWeightedChannels(candidates []weightedChannel) []weightedChannel {
	total := 0.0
	for _, candidate := range candidates {
		if candidate.rawWeight > 0 {
			total += candidate.rawWeight
		}
	}
	if total <= 0 {
		return nil
	}
	normalized := make([]weightedChannel, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.rawWeight <= 0 {
			continue
		}
		candidate.normalizedWeight = candidate.rawWeight / total
		normalized = append(normalized, candidate)
	}
	return normalized
}
