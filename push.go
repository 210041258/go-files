// Package push provides a generic stack data structure.
// It supports typical stack operations: Push, Pop, Peek, Len, and IsEmpty.
package testutils

// Stack is a LIFO (last‑in, first‑out) data structure.
type Stack[T any] struct {
	items []T
}

// NewStack creates a new empty stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

// Push adds an item to the top of the stack.
func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

// Pop removes and returns the top item from the stack.
// If the stack is empty, it returns the zero value and false.
func (s *Stack[T]) Pop() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item, true
}

// Peek returns the top item without removing it.
// If the stack is empty, it returns the zero value and false.
func (s *Stack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Len returns the number of items in the stack.
func (s *Stack[T]) Len() int {
	return len(s.items)
}

// IsEmpty reports whether the stack is empty.
func (s *Stack[T]) IsEmpty() bool {
	return len(s.items) == 0
}

// Clear removes all items from the stack.
func (s *Stack[T]) Clear() {
	s.items = nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     stack := push.NewStack[int]()
//     stack.Push(10)
//     stack.Push(20)
//     val, ok := stack.Pop() // 20, true
//     fmt.Println(val, ok)
//     fmt.Println(stack.Len()) // 1
// }