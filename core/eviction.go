package core

import "redis_internals/config"

// Evict the first key it found while iterating the map
func evictFirst() {
	for k := range store {
		delete(store, k)
		return
	}
}

// Can add mutilple eviction strategies
// e.g. LRU, LFU, approximated LRU(multiple samples and compare time), aproximated LFU(morris counter)
func evict() {
	switch config.EvictionStrategy {
	case "simple-first":
		evictFirst()
	}
}
