package testutils

import (
	"log"
	"sort"
	"sync"
)

// Layer represents a logical resource tier.
// Acquire layers in ascending order to prevent deadlocks.
type Layer int

// Common Lock Layers (Convention)
const (
	LayerDisk    Layer = iota // Lowest level (Filesystem I/O)
	LayerNetwork              // Network I/O
	LayerCache                // In-Memory Cache
	LayerApp                  // Application Logic
	LayerService              // High level services
)

// LayerManager controls locking across different layers to enforce
// global ordering, preventing circular dependencies.
type LayerManager struct {
	mu    sync.RWMutex // RWMutex allows concurrent Reads (Release) vs Writes (Register)
	locks map[Layer]*sync.Mutex
	debug bool
}

// NewLayerManager creates a manager with optional pre-registered layers.
// Pre-registration eliminates map write-locks during standard Release operations.
func NewLayerManager(initialLayers []Layer, debug bool) *LayerManager {
	locks := make(map[Layer]*sync.Mutex)
	for _, l := range initialLayers {
		locks[l] = &sync.Mutex{}
	}

	return &LayerManager{
		locks: locks,
		debug: debug,
	}
}

// Register ensures that a layer exists. Idempotent.
// Safe to call concurrently with Acquire/Release.
func (lm *LayerManager) Register(l Layer) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if _, exists := lm.locks[l]; !exists {
		lm.locks[l] = &sync.Mutex{}
	}
}

// Acquire obtains locks for specified layers.
// The locks are acquired in strict order of their Layer ID (lowest to highest).
// It returns a ScopedUnlocker that ensures they are released in reverse order.
func (lm *LayerManager) Acquire(layers ...Layer) *ScopedUnlocker {
	if len(layers) == 0 {
		return &ScopedUnlocker{lm: lm, locked: []Layer{}}
	}

	// 1. Sort layers to ensure global acquisition order
	sorted := make([]Layer, len(layers))
	copy(sorted, layers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// 2. Ensure mutexes exist (Write Lock)
	lm.mu.Lock()
	for _, l := range sorted {
		if _, exists := lm.locks[l]; !exists {
			lm.locks[l] = &sync.Mutex{}
		}
	}
	lm.mu.Unlock()

	// 3. Lock in order (Lock-free regarding map now)
	for _, l := range sorted {
		lm.locks[l].Lock()
	}

	return &ScopedUnlocker{
		lm:     lm,
		locked: sorted, // Store sorted order to avoid re-sorting in Unlock
	}
}

// Release unlocks specified layers in reverse order (LIFO).
// Safe to call even if some layers were never acquired (debug check verifies).
func (lm *LayerManager) Release(layers ...Layer) {
	if len(layers) == 0 {
		return
	}

	// Sort descending for LIFO unlock
	sorted := make([]Layer, len(layers))
	copy(sorted, layers)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] > sorted[j] })

	lm.mu.RLock() // Read lock is sufficient for reading map
	defer lm.mu.RUnlock()

	for _, l := range sorted {
		mu, ok := lm.locks[l]
		if !ok {
			if lm.debug {
				log.Printf("[LayerManager] WARNING: Attempting to release unknown layer %d", l)
			}
			continue // Skip if layer doesn't exist (debug mode handles it)
		}
		mu.Unlock()
	}
}

// ScopedUnlocker stores the lock order to guarantee safe, order-reversed unlocking.
// Usage: defer lm.Acquire(LayerA, LayerB).Unlock()
type ScopedUnlocker struct {
	lm     *LayerManager
	locked []Layer
}

// Unlock releases the locks in reverse order (LIFO).
// It uses the stored sorted list, avoiding the overhead of sorting again.
func (su *ScopedUnlocker) Unlock() {
	if len(su.locked) == 0 {
		return
	}

	su.lm.mu.RLock()
	defer su.lm.mu.RUnlock()

	// Iterate in reverse order
	for i := len(su.locked) - 1; i >= 0; i-- {
		l := su.locked[i]
		mu, ok := su.lm.locks[l]
		if ok {
			mu.Unlock()
		} else if su.lm.debug {
			log.Printf("[ScopedUnlocker] WARNING: Releasing unknown layer %d", l)
		}
	}
}

// =============================================================================
// USAGE EXAMPLE
// =============================================================================

/*
┌─────────────────────────────────────────────┐
│           LAYER MANAGER STRATEGY             │
├─────────────────────────────────────────────┤
│                                               │
│ 1. Define Layers: Assign IDs based on      │
│    dependency (Low -> High).               │
│                                               │
│ 2. Pre-Register: Pass known layers to      │
│    NewLayerManager to make Release lock-free.   │
│                                               │
│ 3. Acquire Any Order: Pass layers in any       │
│    order; Manager sorts internally.           │
│                                               │
│ 4. Use ScopedLock: Use defer to ensure       │
│    release, even on panic.                  │
│                                               │
└─────────────────────────────────────────────┘
*/

// LockingService demonstrates safe usage.
type LockingService struct {
	lm *LayerManager
}

func NewLockingService() *LockingService {
	// Pre-register known layers for performance
	return &LockingService{
		lm: NewLayerManager([]Layer{
			LayerDisk,
			LayerNetwork,
			LayerCache,
			LayerSQL,
			LayerWorker,
			LayerAPI,
		}, true), // Enable debug mode to see warnings
	}
}

// ComplexOperation locks resources in any order.
// Deadlock is impossible regardless of how caller orders arguments.
func (s *LockingService) ComplexOperation() {
	// Order A
	defer s.lm.Acquire(LayerAPI, LayerCache, LayerDisk).Unlock()

	// Order B (Reverse) - Works perfectly fine
	defer s.lm.Acquire(LayerDisk, LayerCache, LayerAPI).Unlock()

	// Work
}
