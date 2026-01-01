package main

// Sample represents a key-value pair sampled from the database for eviction purposes.
// This structure is used to pass sampled keys along with their associated items
// to eviction algorithms that need to examine or remove keys.
//
// Fields:
//   - key: The string key from the database
//   - value: A pointer to the Item associated with this key
type Sample struct {
	key   string
	value *Item
}

// sampleKeysRandom randomly samples a subset of keys from the database for eviction algorithms.
// This function is used by eviction policies (like AllKeysRandom) to select candidate
// keys for removal when memory limits are reached.
//
// Parameters:
//   - state: The application state containing configuration for the number of samples
//     to collect (maxmemorySamples)
//
// Returns:
//   - []Sample: A slice of Sample structures containing up to maxmemorySamples key-value pairs
//     Returns fewer samples if the database has fewer keys than requested
//
// Behavior:
//  1. Reads maxmemorySamples from state.config to determine how many keys to sample
//  2. Iterates through DB.store map (Go maps have random iteration order)
//  3. Collects key-value pairs into Sample structures
//  4. Stops when the requested number of samples is reached or all keys are examined
//  5. Returns the collected samples
//
// Sampling Strategy:
//   - Uses Go's built-in map iteration which provides pseudo-random order
//   - No explicit randomization needed - map iteration order is intentionally non-deterministic
//   - Efficient: O(n) where n is min(numSamples, totalKeys)
//   - Stops early if enough samples collected
//
// Thread Safety:
//   - This function directly accesses DB.store without locking
//   - Should be called while holding appropriate locks (read lock for sampling)
//   - Typically called from EvictKeys() which manages its own locking
//
// Configuration:
//   - Number of samples controlled by maxmemory-samples config directive
//   - Default: 5 samples (if not configured or invalid value)
//   - More samples = better eviction decisions but higher CPU cost
//   - Fewer samples = faster but potentially less optimal eviction
//
// Use Cases:
//   - Eviction algorithms need candidate keys to evaluate for removal
//   - Random sampling avoids scanning entire database (performance optimization)
//   - Used by AllKeysRandom eviction policy
//
// Example:
//
//	// In EvictKeys() function:
//	samples := sampleKeysRandom(state)  // Get 5 random keys (if maxmemory-samples=5)
//	for _, sample := range samples {
//	    // Evaluate or remove sample.key
//	}
//
// Note:
//   - The function does not acquire locks - caller must ensure thread safety
//   - Map iteration order in Go is random but not cryptographically secure
//   - For very small databases, may return all keys if fewer than maxmemorySamples
func sampleKeysRandom(state *AppState) []Sample {

	numSamples := state.config.maxmemorySamples
	keys := make([]Sample, 0, numSamples) // size, room
	for key, item := range DB.store {     // maps are random order
		keys = append(keys, Sample{key: key, value: item})
		if len(keys) >= numSamples {
			break
		}
	}
	return keys
}
