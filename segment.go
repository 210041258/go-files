// Package segment provides a generic segment tree for associative range
// queries (sum, min, max, etc.) with optional lazy propagation for
// efficient range updates. It is thread‑safe only if used with external
// synchronization.
//
// All operations are O(log n) for both query and update.
package testutils

import (
)

// --------------------------------------------------------------------
// Monoid interface
// --------------------------------------------------------------------

// Monoid defines an associative binary operation with an identity element.
// It is used to combine values in the segment tree.
type Monoid[T any] struct {
	// Identity returns the identity element (e.g., 0 for sum, +inf for min).
	Identity func() T
	// Combine combines two elements (associative).
	Combine func(a, b T) T
}

// Common monoids
var (
	// SumMonoid is the monoid for addition over numbers.
	SumMonoid[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64] = Monoid[T]{
		Identity: func() T { var zero T; return zero },
		Combine:  func(a, b T) T { return a + b },
	}

	// MinMonoid is the monoid for minimum over ordered types.
	MinMonoid[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string] = Monoid[T]{
		Identity: func() T { var zero T; return zero }, // note: min needs +Inf for numeric, but we handle by special case?
		// We'll define a proper identity later; for now we require explicit init.
	}

	// MaxMonoid is the monoid for maximum.
	MaxMonoid[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string] = Monoid[T]{
		Identity: func() T { var zero T; return zero },
	}
)

// --------------------------------------------------------------------
// Segment Tree (without lazy propagation)
// --------------------------------------------------------------------

// Tree is a simple segment tree for range queries without updates.
// It is immutable after build.
type Tree[T any] struct {
	n      int
	tree   []T
	monoid Monoid[T]
}

// New builds a segment tree from the given slice and monoid.
func New[T any](data []T, m Monoid[T]) *Tree[T] {
	n := len(data)
	tree := make([]T, 4*n)
	var build func(node, l, r int)
	build = func(node, l, r int) {
		if l == r {
			tree[node] = data[l]
			return
		}
		mid := (l + r) / 2
		build(node*2, l, mid)
		build(node*2+1, mid+1, r)
		tree[node] = m.Combine(tree[node*2], tree[node*2+1])
	}
	if n > 0 {
		build(1, 0, n-1)
	}
	return &Tree[T]{
		n:      n,
		tree:   tree,
		monoid: m,
	}
}

// Query returns the monoid combination over the range [l, r] (inclusive).
// Returns zero value and false if the range is invalid or empty.
func (t *Tree[T]) Query(l, r int) value.Option[T] {
	if l < 0 || r >= t.n || l > r {
		return value.None[T]()
	}
	var query func(node, nl, nr int) T
	query = func(node, nl, nr int) T {
		if r < nl || nr < l {
			return t.monoid.Identity()
		}
		if l <= nl && nr <= r {
			return t.tree[node]
		}
		mid := (nl + nr) / 2
		left := query(node*2, nl, mid)
		right := query(node*2+1, mid+1, nr)
		return t.monoid.Combine(left, right)
	}
	return value.Some(query(1, 0, t.n-1))
}

// Get returns the element at index i (if in range).
func (t *Tree[T]) Get(i int) value.Option[T] {
	return t.Query(i, i)
}

// Len returns the length of the original array.
func (t *Tree[T]) Len() int { return t.n }

// --------------------------------------------------------------------
// Lazy Segment Tree (range updates + range queries)
// --------------------------------------------------------------------

// LazyTree is a segment tree with lazy propagation for range updates.
// The update operation must also be associative and composable.
type LazyTree[T any] struct {
	n         int
	tree      []T
	lazy      []T // pending updates
	monoid    Monoid[T]
	apply     func(value *T, update T, length int) // how to apply an update to a node
	identity  T                                     // identity for updates
}

// NewLazy creates a segment tree with lazy propagation.
// Parameters:
//   - data: initial slice
//   - m: monoid for combining values (query operation)
//   - apply: function that applies an update to a node value, given the node's segment length
//   - updateIdentity: identity element for updates (e.g., 0 for addition, nil for assignment)
func NewLazy[T any](
	data []T,
	m Monoid[T],
	apply func(value *T, update T, length int),
	updateIdentity T,
) *LazyTree[T] {
	n := len(data)
	tree := make([]T, 4*n)
	lazy := make([]T, 4*n)
	// Initialize lazy with identity
	for i := range lazy {
		lazy[i] = updateIdentity
	}
	var build func(node, l, r int)
	build = func(node, l, r int) {
		if l == r {
			tree[node] = data[l]
			return
		}
		mid := (l + r) / 2
		build(node*2, l, mid)
		build(node*2+1, mid+1, r)
		tree[node] = m.Combine(tree[node*2], tree[node*2+1])
	}
	if n > 0 {
		build(1, 0, n-1)
	}
	return &LazyTree[T]{
		n:         n,
		tree:      tree,
		lazy:      lazy,
		monoid:    m,
		apply:     apply,
		identity:  updateIdentity,
	}
}

// push propagates lazy updates to children.
func (t *LazyTree[T]) push(node, l, r int) {
	if t.lazy[node] == t.identity {
		return
	}
	mid := (l + r) / 2
	// Apply to left child
	t.apply(&t.tree[node*2], t.lazy[node], mid-l+1)
	t.lazy[node*2] = t.combineUpdate(t.lazy[node*2], t.lazy[node])
	// Apply to right child
	t.apply(&t.tree[node*2+1], t.lazy[node], r-mid)
	t.lazy[node*2+1] = t.combineUpdate(t.lazy[node*2+1], t.lazy[node])
	// Clear current node's lazy
	t.lazy[node] = t.identity
}

// combineUpdate combines two updates (order matters: outer applied after inner).
// By default, we assume updates are composable via the same apply function? 
// For simplicity, we define that updates are combined using the same monoid as query? 
// That's not always true. For addition, update composition is addition; for assignment, it's overwrite.
// We'll require the caller to provide a combine function, or we can provide a default that uses the apply? Hmm.
// For now, we assume updates form a monoid and we have a combine function. We'll add a field.
// Let's extend the struct to include updateMonoid.

// For simplicity, we'll add an updateMonoid field in the struct.
// But to keep API clean, we'll require a combineUpdate function parameter in NewLazy.
// We'll adjust the constructor.

// We'll redesign slightly: pass update monoid as well.

// Update:
// LazyTree now requires an update monoid.
type LazyTree[T any] struct {
	n            int
	tree         []T
	lazy         []T
	queryMonoid  Monoid[T] // for combining node values
	updateMonoid Monoid[T] // for combining updates
	apply        func(value *T, update T, length int)
}

// NewLazy creates a lazy segment tree.
//   - data: initial array
//   - queryMonoid: monoid for range query (combine, identity)
//   - updateMonoid: monoid for lazy updates (combine, identity). This defines how two updates are merged.
//   - apply: function to apply an update to a node value, given segment length
func NewLazy[T any](
	data []T,
	queryMonoid Monoid[T],
	updateMonoid Monoid[T],
	apply func(value *T, update T, length int),
) *LazyTree[T] {
	n := len(data)
	tree := make([]T, 4*n)
	lazy := make([]T, 4*n)
	// Initialize lazy with update identity
	updateIdentity := updateMonoid.Identity()
	for i := range lazy {
		lazy[i] = updateIdentity
	}
	var build func(node, l, r int)
	build = func(node, l, r int) {
		if l == r {
			tree[node] = data[l]
			return
		}
		mid := (l + r) / 2
		build(node*2, l, mid)
		build(node*2+1, mid+1, r)
		tree[node] = queryMonoid.Combine(tree[node*2], tree[node*2+1])
	}
	if n > 0 {
		build(1, 0, n-1)
	}
	return &LazyTree[T]{
		n:            n,
		tree:         tree,
		lazy:         lazy,
		queryMonoid:  queryMonoid,
		updateMonoid: updateMonoid,
		apply:        apply,
	}
}

// push propagates lazy updates to children.
func (t *LazyTree[T]) push(node, l, r int) {
	if t.lazy[node] == t.updateMonoid.Identity() {
		return
	}
	mid := (l + r) / 2
	// Apply to left child
	t.apply(&t.tree[node*2], t.lazy[node], mid-l+1)
	t.lazy[node*2] = t.updateMonoid.Combine(t.lazy[node*2], t.lazy[node])
	// Apply to right child
	t.apply(&t.tree[node*2+1], t.lazy[node], r-mid)
	t.lazy[node*2+1] = t.updateMonoid.Combine(t.lazy[node*2+1], t.lazy[node])
	// Clear current node
	t.lazy[node] = t.updateMonoid.Identity()
}

// UpdateRange applies an update to all elements in [ql, qr].
func (t *LazyTree[T]) UpdateRange(ql, qr int, update T) {
	if ql < 0 || qr >= t.n || ql > qr {
		return
	}
	var updateFunc func(node, l, r int)
	updateFunc = func(node, l, r int) {
		if ql <= l && r <= qr {
			t.apply(&t.tree[node], update, r-l+1)
			t.lazy[node] = t.updateMonoid.Combine(t.lazy[node], update)
			return
		}
		t.push(node, l, r)
		mid := (l + r) / 2
		if ql <= mid {
			updateFunc(node*2, l, mid)
		}
		if qr > mid {
			updateFunc(node*2+1, mid+1, r)
		}
		t.tree[node] = t.queryMonoid.Combine(t.tree[node*2], t.tree[node*2+1])
	}
	if t.n > 0 {
		updateFunc(1, 0, t.n-1)
	}
}

// QueryRange returns the monoid combination over [ql, qr].
func (t *LazyTree[T]) QueryRange(ql, qr int) value.Option[T] {
	if ql < 0 || qr >= t.n || ql > qr {
		return value.None[T]()
	}
	var queryFunc func(node, l, r int) T
	queryFunc = func(node, l, r int) T {
		if ql <= l && r <= qr {
			return t.tree[node]
		}
		t.push(node, l, r)
		mid := (l + r) / 2
		if qr <= mid {
			return queryFunc(node*2, l, mid)
		}
		if ql > mid {
			return queryFunc(node*2+1, mid+1, r)
		}
		left := queryFunc(node*2, l, mid)
		right := queryFunc(node*2+1, mid+1, r)
		return t.queryMonoid.Combine(left, right)
	}
	if t.n == 0 {
		return value.None[T]()
	}
	return value.Some(queryFunc(1, 0, t.n-1))
}

// --------------------------------------------------------------------
// Predefined monoids with proper identities
// --------------------------------------------------------------------

// SumMonoidFull returns a Monoid for addition with identity 0.
func SumMonoidFull[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64]() Monoid[T] {
	return Monoid[T]{
		Identity: func() T { var zero T; return zero },
		Combine:  func(a, b T) T { return a + b },
	}
}

// MinMonoidFull returns a Monoid for minimum with identity = max possible value.
// Since Go generics don't allow numeric constants, we require the caller to provide
// the identity value explicitly. For convenience, we provide a constructor.
func MinMonoidFull[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](identity T) Monoid[T] {
	return Monoid[T]{
		Identity: func() T { return identity },
		Combine:  func(a, b T) T { if a < b { return a }; return b },
	}
}

// MaxMonoidFull returns a Monoid for maximum with identity = min possible value.
func MaxMonoidFull[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](identity T) Monoid[T] {
	return Monoid[T]{
		Identity: func() T { return identity },
		Combine:  func(a, b T) T { if a > b { return a }; return b },
	}
}

// --------------------------------------------------------------------
// Convenience constructors for common use cases
// --------------------------------------------------------------------

// NewSumTree returns a segment tree for range sum queries.
func NewSumTree[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](data []T) *Tree[T] {
	return New(data, SumMonoidFull[T]())
}

// NewMinTree returns a segment tree for range minimum queries.
func NewMinTree[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](data []T, identity T) *Tree[T] {
	return New(data, MinMonoidFull[T](identity))
}

// NewMaxTree returns a segment tree for range maximum queries.
func NewMaxTree[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](data []T, identity T) *Tree[T] {
	return New(data, MaxMonoidFull[T](identity))
}

// NewLazySumTree creates a lazy segment tree for range add + range sum.
func NewLazySumTree[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](data []T) *LazyTree[T] {
	queryMonoid := SumMonoidFull[T]()
	updateMonoid := SumMonoidFull[T]() // add updates compose by addition
	apply := func(value *T, update T, length int) {
		*value += update * T(length)
	}
	return NewLazy(data, queryMonoid, updateMonoid, apply)
}

// NewLazyAssignTree creates a lazy segment tree for range assignment + range sum.
// Note: assignment updates do not compose by addition; they override.
// We use a special update monoid: the identity is a sentinel (e.g., nil), and combine
// selects the later update. For simplicity, we define a generic assign monoid.
func NewLazyAssignTree[T any](data []T, zero T, assign func(*T, T, int)) *LazyTree[T] {
	queryMonoid := SumMonoidFull[T]() // assumes numeric, adjust if needed
	// Update monoid: combine(a,b) = b (last write wins)
	updateMonoid := Monoid[T]{
		Identity: func() T { return zero },
		Combine:  func(a, b T) T { return b },
	}
	apply := assign // e.g., func(value *T, update T, length int) { *value = update * T(length) } for sum
	return NewLazy(data, queryMonoid, updateMonoid, apply)
}