// Package pop provides a generic queue (FIFO) data structure.
// It supports enqueue (push) and dequeue (pop) operations.
package pop

// Queue is a FIFO (first‑in, first‑out) data structure.
type Queue[T any] struct {
	items []T
}

// NewQueue creates a new empty queue.
func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{}
}

// Enqueue adds an item to the back of the queue.
func (q *Queue[T]) Enqueue(item T) {
	q.items = append(q.items, item)
}

// Dequeue removes and returns the front item from the queue.
// If the queue is empty, it returns the zero value and false.
func (q *Queue[T]) Dequeue() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// Peek returns the front item without removing it.
// If the queue is empty, it returns the zero value and false.
func (q *Queue[T]) Peek() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	return q.items[0], true
}

// Len returns the number of items in the queue.
func (q *Queue[T]) Len() int {
	return len(q.items)
}

// IsEmpty reports whether the queue is empty.
func (q *Queue[T]) IsEmpty() bool {
	return len(q.items) == 0
}

// Clear removes all items from the queue.
func (q *Queue[T]) Clear() {
	q.items = nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     q := pop.NewQueue[string]()
//     q.Enqueue("first")
//     q.Enqueue("second")
//
//     item, ok := q.Dequeue() // "first", true
//     fmt.Println(item, ok)
//     fmt.Println(q.Len()) // 1
// }