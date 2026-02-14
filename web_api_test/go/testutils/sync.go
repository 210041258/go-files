package testutils

import (
	"sync"
	"sync/atomic"
)

// =============================================================================
// 1. SAFE MAP (Generic Thread-Safe Map)
// =============================================================================

// SafeMap is a wrapper around map[K]V providing concurrent access safety.
// It uses sync.RWMutex to allow multiple readers or a single writer.
type SafeMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// NewSafeMap creates a new SafeMap with optional initial capacity.
func NewSafeMap[K comparable, V any](capacity int) *SafeMap[K, V] {
	return &SafeMap[K, V]{
		data: make(map[K]V, capacity),
	}
}

// Load retrieves the value for a key.
// The ok result indicates whether the key was found.
func (m *SafeMap[K, V]) Load(key K) (value V, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

// Store sets the value for a key.
func (m *SafeMap[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result reports whether the value was loaded (true) or stored (false).
func (m *SafeMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	actual, loaded := m.data[key]
	if loaded {
		return actual, true
	}
	m.data[key] = value
	return value, false
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *SafeMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, loaded := m.data[key]
	if loaded {
		delete(m.data, key)
	}
	return val, loaded
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
// Note: This holds a Read Lock for the duration of the iteration.
func (m *SafeMap[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}

// Keys returns a slice of all keys in the map.
// Note: The order of keys is randomized by Go runtime.
func (m *SafeMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of items in the map.
func (m *SafeMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Delete removes the key from the map.
func (m *SafeMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// Clear removes all entries from the map.
func (m *SafeMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Reallocation ensures the backing array is freed.
	m.data = make(map[K]V, len(m.data))
}

// =============================================================================
// 2. ATOMIC COUNTER (Lock-Free Concurrency)
// =============================================================================

// AtomicCounter is a lock-free counter based on sync/atomic.
// It is significantly faster than a Mutex-wrapped int for high-frequency increments.
type AtomicCounter struct {
	value int64
}

// Add adds the delta to the counter and returns the new value.
func (ac *AtomicCounter) Add(delta int64) int64 {
	return atomic.AddInt64(&ac.value, delta)
}

// Increment adds 1 to the counter and returns the new value.
func (ac *AtomicCounter) Increment() int64 {
	return atomic.AddInt64(&ac.value, 1)
}

// Decrement subtracts 1 from the counter and returns the new value.
func (ac *AtomicCounter) Decrement() int64 {
	return atomic.AddInt64(&ac.value, -1)
}

// Get returns the current value of the counter.
func (ac *AtomicCounter) Get() int64 {
	return atomic.LoadInt64(&ac.value)
}

// Reset sets the counter to 0.
func (ac *AtomicCounter) Reset() {
	atomic.StoreInt64(&ac.value, 0)
}

// Swap atomically stores new into the counter and returns the old value.
func (ac *AtomicCounter) Swap(new int64) (old int64) {
	return atomic.SwapInt64(&ac.value, new)
}

// CompareAndSwap executes the compare-and-swap operation.
func (ac *AtomicCounter) CompareAndSwap(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&ac.value, old, new)
}

// =============================================================================
// 3. GENERIC OBJECT POOL (sync.Pool Wrapper)
// =============================================================================

// Pool manages a pool of generic instances T to avoid repeated allocations.
// It is safe for concurrent use.
//
// IMPORTANT: Objects retrieved from the pool may contain stale data.
// The caller is responsible for resetting the object to a clean state before use.
type Pool[T any] struct {
	p *sync.Pool
}

// NewPool creates a new Pool with a function to generate new values when empty.
func NewPool[T any](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		p: &sync.Pool{
			New: func() interface{} {
				return newFunc()
			},
		},
	}
}

// Get selects an arbitrary item from the Pool, removes it from the Pool,
// and returns it to the caller.
// If the Pool is empty, newFunc is called to create a new instance.
func (p *Pool[T]) Get() T {
	return p.p.Get().(T)
}

// Put adds x to the pool.
func (p *Pool[T]) Put(x T) {
	p.p.Put(x)
}

// =============================================================================
// 4. SINGLETON / ONCE PATTERN
// =============================================================================

// Singleton manages a single instance of T created exactly once.
type Singleton[T any] struct {
	once sync.Once
	val  T
}

// NewSingleton creates a wrapper for lazy initialization.
func NewSingleton[T any](initFunc func() T) *Singleton[T] {
	return &Singleton[T]{}
}

// Get returns the instance, initializing it on the first call.
// It is safe for concurrent use.
func (s *Singleton[T]) Get(initFunc func() T) T {
	s.once.Do(func() {
		s.val = initFunc()
	})
	return s.val
}

// =============================================================================
// 5. BROADCASTER (sync.Cond Pattern)
// =============================================================================

// Broadcaster allows one goroutine to signal multiple waiting goroutines.
// Useful for "State Changed" notifications (Fan-Out).
// Once Signal() is called, Wait() returns immediately for all future callers.
type Broadcaster struct {
	mu   sync.Mutex
	cond *sync.Cond
	done bool
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster() *Broadcaster {
	b := &Broadcaster{}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Signal releases all waiting goroutines.
// Subsequent calls to Wait() will return immediately.
func (b *Broadcaster) Signal() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done = true
	b.cond.Broadcast() // Wake everyone
}

// Wait blocks until Signal is called.
// It returns immediately if Signal has already been called.
func (b *Broadcaster) Wait() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for !b.done {
		b.cond.Wait()
	}
}

// =============================================================================
// 6. LATCH (Count-Down Latch via Channel)
// =============================================================================

// Latch is a synchronization primitive that blocks until a specific number
// of "Arrive" calls have been reached, after which it stays permanently open.
// This is superior to polling implementations as it uses channel blocking.
type Latch struct {
	threshold int64
	count     int64
	done      chan struct{}
	once      sync.Once
}

// NewLatch creates a new Latch that waits for 'n' arrivals.
// If n <= 0, the latch is immediately open.
func NewLatch(n int) *Latch {
	l := &Latch{
		threshold: int64(n),
		done:      make(chan struct{}),
	}
	if n <= 0 {
		close(l.done)
	}
	return l
}

// Arrive signals one event. If the threshold is reached, the latch opens.
// Arrive is safe for concurrent calls.
func (l *Latch) Arrive() {
	if atomic.AddInt64(&l.count, 1) >= l.threshold {
		l.once.Do(func() {
			close(l.done)
		})
	}
}

// Wait blocks until the threshold is reached.
// If the latch is already open, it returns immediately.
func (l *Latch) Wait() {
	<-l.done
}

// Done returns a channel that is closed when the latch opens.
// This allows using the latch in select statements.
func (l *Latch) Done() <-chan struct{} {
	return l.done
}

// =============================================================================
// 7. USAGE EXAMPLES & BEST PRACTICES
// =============================================================================

/*
┌─────────────────────────────────────────────────────┐
│           SYNC PACKAGE BEST PRACTICES                │
├─────────────────────────────────────────────────────┤
│                                                      │
│ 1. NEVER copy a sync.Mutex or sync.RWMutex.      │
│    Always use pointers.                             │
│                                                      │
│ 2. Always defer Unlock() immediately after Lock().   │
│    This prevents deadlocks on panics.             │
│                                                      │
│ 3. Prefer Atomic (int64, bool) over Mutex for    │
│    simple counters and flags.                      │
│                                                      │
│ 4. Use sync.RWMutex when reads vastly outnumber     │
│    writes.                                         │
│                                                      │
│ 5. Use sync.Pool for temporary buffers (e.g.,      │
│    bytes.Buffer, []byte) to reduce GC.           │
│                                                      │
│ 6. Always reset objects from sync.Pool before use.   │
│    Data from previous use persists.              │
│                                                      │
└─────────────────────────────────────────────────────┘
*/

// ✅ GOOD: Pointer to Mutex
type GoodService struct {
	mu sync.Mutex
}

func (s *GoodService) DoWork() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// work
}

// ❌ BAD: Copying Mutex
type BadService struct {
	mu sync.Mutex
}

func Work(bad BadService) {
	// bad.mu is a copy, Lock() will panic or have no effect
	bad.mu.Lock()
	defer bad.mu.Unlock()
}

// ✅ GOOD: Atomic Operations
func GoodAtomic() {
	var counter int64
	atomic.AddInt64(&counter, 1)
}

// ❌ BAD: Unnecessary Mutex for simple counter
func BadAtomic() {
	var mu sync.Mutex
	var counter int
	mu.Lock()
	counter++
	mu.Unlock()
}
