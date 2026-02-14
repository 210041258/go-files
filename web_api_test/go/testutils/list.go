// Package list provides a generic doubly linked list.
// It supports typical operations like PushFront, PushBack, PopFront, PopBack,
// and iteration via an Iterator.
package list

// Element is an element of the linked list.
type Element[T any] struct {
	Value T
	next  *Element[T]
	prev  *Element[T]
	list  *List[T]
}

// Next returns the next element or nil.
func (e *Element[T]) Next() *Element[T] {
	if p := e.next; e.list != nil && p != &e.list.head {
		return p
	}
	return nil
}

// Prev returns the previous element or nil.
func (e *Element[T]) Prev() *Element[T] {
	if p := e.prev; e.list != nil && p != &e.list.head {
		return p
	}
	return nil
}

// List represents a doubly linked list.
// The zero value is an empty list ready to use.
type List[T any] struct {
	head Element[T] // sentinel element
	len  int
}

// New creates a new empty list.
func New[T any]() *List[T] {
	l := &List[T]{}
	l.head.next = &l.head
	l.head.prev = &l.head
	return l
}

// Len returns the number of elements in the list.
func (l *List[T]) Len() int { return l.len }

// Front returns the first element of the list or nil if empty.
func (l *List[T]) Front() *Element[T] {
	if l.len == 0 {
		return nil
	}
	return l.head.next
}

// Back returns the last element of the list or nil if empty.
func (l *List[T]) Back() *Element[T] {
	if l.len == 0 {
		return nil
	}
	return l.head.prev
}

// insert inserts e after at, increments len, and returns e.
func (l *List[T]) insert(e, at *Element[T]) *Element[T] {
	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
	e.list = l
	l.len++
	return e
}

// remove removes e from its list, decrements len, and returns e.
func (l *List[T]) remove(e *Element[T]) *Element[T] {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil
	e.prev = nil
	e.list = nil
	l.len--
	return e
}

// PushFront inserts a new element with value v at the front of the list.
func (l *List[T]) PushFront(v T) *Element[T] {
	return l.insert(&Element[T]{Value: v}, &l.head)
}

// PushBack inserts a new element with value v at the back of the list.
func (l *List[T]) PushBack(v T) *Element[T] {
	return l.insert(&Element[T]{Value: v}, l.head.prev)
}

// InsertBefore inserts a new element with value v immediately before mark.
// If mark is not an element of l, the list is not modified.
// The new element is returned.
func (l *List[T]) InsertBefore(v T, mark *Element[T]) *Element[T] {
	if mark.list != l {
		return nil
	}
	return l.insert(&Element[T]{Value: v}, mark.prev)
}

// InsertAfter inserts a new element with value v immediately after mark.
// If mark is not an element of l, the list is not modified.
// The new element is returned.
func (l *List[T]) InsertAfter(v T, mark *Element[T]) *Element[T] {
	if mark.list != l {
		return nil
	}
	return l.insert(&Element[T]{Value: v}, mark)
}

// Remove removes e from the list if it is an element of l.
// It returns the element's value.
// The element must not be nil.
func (l *List[T]) Remove(e *Element[T]) T {
	if e.list == l {
		l.remove(e)
	}
	return e.Value
}

// MoveToFront moves element e to the front of the list.
// If e is not an element of l, the list is not modified.
func (l *List[T]) MoveToFront(e *Element[T]) {
	if e.list != l || l.head.next == e {
		return
	}
	l.insert(l.remove(e), &l.head)
}

// MoveToBack moves element e to the back of the list.
// If e is not an element of l, the list is not modified.
func (l *List[T]) MoveToBack(e *Element[T]) {
	if e.list != l || l.head.prev == e {
		return
	}
	l.insert(l.remove(e), l.head.prev)
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
func (l *List[T]) MoveBefore(e, mark *Element[T]) {
	if e.list != l || mark.list != l || e == mark {
		return
	}
	l.insert(l.remove(e), mark.prev)
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
func (l *List[T]) MoveAfter(e, mark *Element[T]) {
	if e.list != l || mark.list != l || e == mark {
		return
	}
	l.insert(l.remove(e), mark)
}

// ----------------------------------------------------------------------
// Iterator support
// ----------------------------------------------------------------------

// Iterator allows iteration over the list.
type Iterator[T any] struct {
	next *Element[T]
}

// NewIterator returns an iterator that starts at the front of the list.
func (l *List[T]) NewIterator() *Iterator[T] {
	return &Iterator[T]{next: l.Front()}
}

// Next returns the next element and advances the iterator.
// It returns nil when the iteration finishes.
func (it *Iterator[T]) Next() *Element[T] {
	if it.next == nil {
		return nil
	}
	elem := it.next
	it.next = elem.Next()
	return elem
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     l := list.New[int]()
//     l.PushBack(10)
//     l.PushFront(5)
//     l.PushBack(20)
//
//     for e := l.Front(); e != nil; e = e.Next() {
//         fmt.Println(e.Value) // 5, 10, 20
//     }
//
//     e := l.Front().Next() // element with value 10
//     l.Remove(e)
//
//     l.MoveToBack(l.Front()) // move 5 to back
// }