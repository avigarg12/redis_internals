package core

import (
	"redis_internals/config"
	"time"
)

// Evict the first key it found while iterating the map
func evictFirst() {
	for k := range store {
		Del(k)
		return
	}
}

func evictAllKeysRandom() {
	evictCount := int64(config.EvictionRatio * float64(config.KeysLimit))

	for k := range store {
		Del(k)
		evictCount--
		if evictCount <= 0 {
			break
		}
	}
}

/*
 *  The approximated LRU algorithm
 */
func getCurrentClock() uint32 {
	return uint32(time.Now().Unix()) & 0x00FFFFFF
}

func getIdleTime(lastAccessedAt uint32) uint32 {
	c := getCurrentClock()
	if c >= lastAccessedAt {
		return c - lastAccessedAt
	}
	return c + (0x00FFFFFF - lastAccessedAt)
}

func populateEvictionPool() {
	sampleSize := 5
	for k := range store {
		ePool.Push(k, store[k].LastAccessedAt)
		sampleSize--
		if sampleSize == 0 {
			break
		}
	}
}

func evictAllkeysLRU() {
	populateEvictionPool()
	evictCount := int16(config.EvictionRatio * float64(config.KeysLimit))

	for i := 0; i < int(evictCount) && len(ePool.pool) > 0; i++ {
		item := ePool.Pop()
		if item == nil {
			return
		}
		Del(item.key)
	}
}

// Can add mutilple eviction strategies
// e.g. LRU, LFU, approximated LRU(multiple samples and compare time), aproximated LFU(morris counter)
func evict() {
	switch config.EvictionStrategy {
	case "simple-first":
		evictFirst()
	case "allkeys-random":
		evictAllKeysRandom()
	case "allkeys-lru":
		evictAllkeysLRU()
	}
}
