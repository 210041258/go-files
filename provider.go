// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
)

// Provider is a generic interface for things that provide a value of type T.
// This is useful for dependency injection and controlling external inputs in tests.
type Provider[T any] interface {
	// Get returns the current value.
	Get() T
}

// ------------------------------------------------------------------------
// ValueProvider – returns a constant value.
// ------------------------------------------------------------------------

// ValueProvider returns a fixed value each time.
type ValueProvider[T any] struct {
	value T
}

// NewValueProvider creates a provider that always returns the given value.
func NewValueProvider[T any](value T) *ValueProvider[T] {
	return &ValueProvider[T]{value: value}
}

// Get returns the constant value.
func (p *ValueProvider[T]) Get() T {
	return p.value
}

// ------------------------------------------------------------------------
// FuncProvider – calls a function each time.
// ------------------------------------------------------------------------

// FuncProvider calls a user‑supplied function to obtain the value.
type FuncProvider[T any] struct {
	fn func() T
}

// NewFuncProvider creates a provider that calls fn each time Get is called.
func NewFuncProvider[T any](fn func() T) *FuncProvider[T] {
	return &FuncProvider[T]{fn: fn}
}

// Get returns the result of calling the stored function.
func (p *FuncProvider[T]) Get() T {
	return p.fn()
}

// ------------------------------------------------------------------------
// MockProvider – a provider whose value can be changed dynamically.
// ------------------------------------------------------------------------

// MockProvider is a provider that stores a value and allows it to be updated.
// It is safe for concurrent use.
type MockProvider[T any] struct {
	mu    sync.RWMutex
	value T
}

// NewMockProvider creates a provider with the given initial value.
func NewMockProvider[T any](initial T) *MockProvider[T] {
	return &MockProvider[T]{value: initial}
}

// Get returns the current value.
func (p *MockProvider[T]) Get() T {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.value
}

// Set updates the stored value.
func (p *MockProvider[T]) Set(value T) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.value = value
}

// Update applies a function to atomically modify the value.
func (p *MockProvider[T]) Update(fn func(T) T) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.value = fn(p.value)
}

// ------------------------------------------------------------------------
// ProviderSet – a collection of providers that can be resolved together.
// ------------------------------------------------------------------------

// ProviderSet holds multiple providers, typically used for dependency injection.
type ProviderSet struct {
	providers map[string]interface{}
}

// NewProviderSet creates an empty provider set.
func NewProviderSet() *ProviderSet {
	return &ProviderSet{
		providers: make(map[string]interface{}),
	}
}

// Add registers a provider under the given name.
func (ps *ProviderSet) Add(name string, provider interface{}) {
	ps.providers[name] = provider
}

// Get retrieves a provider by name and type. It panics if the provider is missing
// or has the wrong type.
func Get[T any](ps *ProviderSet, name string) Provider[T] {
	if p, ok := ps.providers[name]; ok {
		if typed, ok := p.(Provider[T]); ok {
			return typed
		}
		panic("testutils: provider " + name + " has wrong type")
	}
	panic("testutils: provider " + name + " not found")
}

// ------------------------------------------------------------------------
// Predefined providers for common types (optional, for convenience)
// ------------------------------------------------------------------------

// StringProvider is a provider of strings.
type StringProvider = Provider[string]

// IntProvider is a provider of ints.
type IntProvider = Provider[int]

// FloatProvider is a provider of float64s.
type FloatProvider = Provider[float64]

// BoolProvider is a provider of bools.
type BoolProvider = Provider[bool]