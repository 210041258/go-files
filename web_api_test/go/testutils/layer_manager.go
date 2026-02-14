package testutils

import (
	"sort"
	"sync"
)

// Layer represents a logical resource tier.
// Always acquire layers in ascending order to prevent deadlocks.
type Layer int

// Predefined layers – extend this list as needed.
// The numeric values define the global lock order.
const (
	LayerDisk    Layer = iota // 0: Filesystem I/O
	LayerNetwork              // 1: Network I/O
	LayerSQL                  // 2: Database connections
	LayerCache                // 3: In-memory cache
	LayerWorker               // 4: Background processing
	LayerAPI                  // 5: HTTP handlers / public interfaces
)

// LayerManager enforces global lock ordering.
// All layers must be pre-registered at construction time.
// The internal map is immutable after construction.
type LayerManager struct {
	locks map[Layer]*sync.Mutex
}

// NewLayerManager creates a manager with a fixed set of layers.
// Duplicate layers in the input are silently ignored.
func NewLayerManager(layers ...Layer) *LayerManager {
	// Deduplicate input using a map - O(n)
	uniqueMap := make(map[Layer]struct{}, len(layers))
	for _, l := range layers {
		uniqueMap[l] = struct{}{}
	}

	// Extract keys and sort them (deterministic order)
	sorted := make([]Layer, 0, len(uniqueMap))
	for l := range uniqueMap {
		sorted = append(sorted, l)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Build immutable map of mutexes
	locks := make(map[Layer]*sync.Mutex, len(sorted))
	for _, l := range sorted {
		locks[l] = &sync.Mutex{}
	}

	return &LayerManager{locks: locks}
}

// Acquire locks the specified layers in strict ascending order.
// Duplicate layers in the input are safely ignored (locked only once).
// Returns a ScopedUnlocker that must be deferred.
func (lm *LayerManager) Acquire(layers ...Layer) *ScopedUnlocker {
	if len(layers) == 0 {
		return &ScopedUnlocker{unlockFunc: func() {}}
	}

	// 1. Deduplicate and sort inputs locally
	uniqueLayers := lm.deduplicateAndSort(layers)

	// 2. Lock layers in sorted order
	for _, l := range uniqueLayers {
		lm.locks[l].Lock()
	}

	// 3. Return unlocker that reverses the sorted order (LIFO)
	return &ScopedUnlocker{
		unlockFunc: func() {
			// Unlock in reverse order (LIFO) to avoid waking waiters
			// on high-priority layers before low-priority ones can release them.
			for i := len(uniqueLayers) - 1; i >= 0; i-- {
				lm.locks[uniqueLayers[i]].Unlock()
			}
		},
	}
}

// deduplicateAndSort sorts the input slice and removes duplicates.
// Returns a new slice with unique layers in ascending order.
func (lm *LayerManager) deduplicateAndSort(layers []Layer) []Layer {
	// Use a map to identify duplicates efficiently
	seen := make(map[Layer]struct{}, len(layers))
	unique := make([]Layer, 0, len(layers))

	for _, l := range layers {
		if _, exists := seen[l]; !exists {
			seen[l] = struct{}{}
			unique = append(unique, l)
		}
	}

	// Sort ascending (guarantees lock order)
	sort.Slice(unique, func(i, j int) bool {
		return unique[i] < unique[j]
	})

	return unique
}

// ScopedUnlocker simplifies defer statements.
// Usage: defer lm.Acquire(LayerDisk, LayerAPI).Unlock()
type ScopedUnlocker struct {
	unlockFunc func()
}

// Unlock releases the previously acquired layers.
func (su *ScopedUnlocker) Unlock() {
	su.unlockFunc()
}

// -----------------------------------------------------------------------------
// INTROSPECTION HELPERS
// -----------------------------------------------------------------------------

// IsLayerRegistered checks if a layer was registered with this manager.
func (lm *LayerManager) IsLayerRegistered(l Layer) bool {
	_, ok := lm.locks[l]
	return ok
}

// RegisteredLayers returns a sorted copy of all registered layers.
// The returned slice is safe for modification by the caller.
func (lm *LayerManager) RegisteredLayers() []Layer {
	layers := make([]Layer, 0, len(lm.locks))
	for l := range lm.locks {
		layers = append(layers, l)
	}
	sort.Slice(layers, func(i, j int) bool {
		return layers[i] < layers[j]
	})
	return layers
}

// =============================================================================
// USAGE EXAMPLE & BEST PRACTICES
// =============================================================================

/*
┌─────────────────────────────────────────────┐
│           LAYER MANAGER STRATEGY             │
├─────────────────────────────────────────────┤
│                                              │
│ 1. Global Ordering: Define IDs once, keep them   │
│    constant (0..N).                             │
│                                              │
│ 2. Strict Constructor: Pass ALL known layers    │
│    to NewLayerManager once at startup.           │
│    The internal map becomes immutable.             │
│                                              │
│ 3. Any Order Input: You can call:               │
│    lm.Acquire(LayerAPI, LayerDisk)               │
│    Manager sorts 0..N internally.               │
│                                              │
│ 4. Duplicates Safe:                            │
│    lm.Acquire(LayerDisk, LayerDisk)               │
│    Locks Disk exactly once.                      │
│                                              │
│ 5. LIFO Unlock:                               │
│    Higher layers unlock before lower layers.         │
│                                              │
└─────────────────────────────────────────────┘
*/

// LockingService demonstrates safe usage.
type LockingService struct {
	lm *LayerManager
}

func NewLockingService() *LockingService {
	return &LockingService{
		lm: NewLayerManager(
			LayerDisk,
			LayerNetwork,
			LayerCache,
			LayerSQL,
			LayerWorker,
			LayerAPI,
		),
	}
}

// ProcessData locks resources in any order.
// Deadlock is impossible regardless of argument order.
func (s *LockingService) ProcessData() {
	// Order A
	defer s.lm.Acquire(LayerAPI, LayerDisk, LayerSQL).Unlock()

	// Critical Section: We hold locks 5, 0, 2 in order 0, 2, 5
}

// AnotherMethod demonstrates locking in reverse order.
func (s *LockingService) AnotherMethod() {
	// Order B (Still Safe)
	defer s.lm.Acquire(LayerSQL, LayerAPI).Unlock()
}
