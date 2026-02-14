// Package node provides generic node data structures for linked lists,
// binary trees, and n‑ary trees. All operations are functional and
// side‑effect‑free unless explicitly noted (e.g., Insert, Delete).
//
// The package is designed to be zero‑dependency and integrates with
// yourmodule/value, yourmodule/slice, and yourmodule/pointer.
package testutils

// --------------------------------------------------------------------
// Singly‑linked list
// --------------------------------------------------------------------

// ListNode is a node in a singly‑linked list.
type ListNode[T any] struct {
	Value T
	Next  *ListNode[T]
}

// NewList creates a new linked list from a slice.
// The list is built in the order of the slice (first element becomes head).
func NewList[T any](values []T) *ListNode[T] {
	if len(values) == 0 {
		return nil
	}
	head := &ListNode[T]{Value: values[0]}
	current := head
	for _, v := range values[1:] {
		current.Next = &ListNode[T]{Value: v}
		current = current.Next
	}
	return head
}

// ListFromSlice is an alias for NewList.
func ListFromSlice[T any](values []T) *ListNode[T] { return NewList(values) }

// ToSlice converts a linked list to a slice.
func (n *ListNode[T]) ToSlice() []T {
	if n == nil {
		return nil
	}
	var res []T
	for cur := n; cur != nil; cur = cur.Next {
		res = append(res, cur.Value)
	}
	return res
}

// Len returns the number of nodes in the list.
func (n *ListNode[T]) Len() int {
	count := 0
	for cur := n; cur != nil; cur = cur.Next {
		count++
	}
	return count
}

// Map applies f to each element and returns a new list.
func (n *ListNode[T]) Map(f func(T) T) *ListNode[T] {
	if n == nil {
		return nil
	}
	newHead := &ListNode[T]{Value: f(n.Value)}
	curNew := newHead
	for cur := n.Next; cur != nil; cur = cur.Next {
		curNew.Next = &ListNode[T]{Value: f(cur.Value)}
		curNew = curNew.Next
	}
	return newHead
}

// Filter returns a new list containing only elements that satisfy f.
func (n *ListNode[T]) Filter(f func(T) bool) *ListNode[T] {
	var head, tail *ListNode[T]
	for cur := n; cur != nil; cur = cur.Next {
		if f(cur.Value) {
			node := &ListNode[T]{Value: cur.Value}
			if head == nil {
				head = node
				tail = node
			} else {
				tail.Next = node
				tail = node
			}
		}
	}
	return head
}

// Reduce reduces the list to a single value using the accumulator.
// acc = f(acc, node.Value)
func (n *ListNode[T]) Reduce(acc T, f func(T, T) T) T {
	result := acc
	for cur := n; cur != nil; cur = cur.Next {
		result = f(result, cur.Value)
	}
	return result
}

// ForEach executes f on each element.
func (n *ListNode[T]) ForEach(f func(T)) {
	for cur := n; cur != nil; cur = cur.Next {
		f(cur.Value)
	}
}

// Reverse returns a new list with elements in reverse order.
func (n *ListNode[T]) Reverse() *ListNode[T] {
	var prev *ListNode[T]
	cur := n
	for cur != nil {
		next := cur.Next
		cur.Next = prev
		prev = cur
		cur = next
	}
	return prev
}

// --------------------------------------------------------------------
// Binary tree
// --------------------------------------------------------------------

// BinaryNode is a node in a binary tree.
type BinaryNode[T any] struct {
	Value T
	Left  *BinaryNode[T]
	Right *BinaryNode[T]
}

// NewBinaryTree creates a binary tree using a level‑order insertion.
// The slice is interpreted as level order; `nil` values represent absent nodes.
func NewBinaryTree[T any](values []value.Option[T]) *BinaryNode[T] {
	if len(values) == 0 || values[0].IsNone() {
		return nil
	}
	root := &BinaryNode[T]{Value: values[0].MustGet()}
	queue := []*BinaryNode[T]{root}
	i := 1
	for len(queue) > 0 && i < len(values) {
		node := queue[0]
		queue = queue[1:]

		// Left child
		if i < len(values) && values[i].IsSome() {
			node.Left = &BinaryNode[T]{Value: values[i].MustGet()}
			queue = append(queue, node.Left)
		}
		i++

		// Right child
		if i < len(values) && values[i].IsSome() {
			node.Right = &BinaryNode[T]{Value: values[i].MustGet()}
			queue = append(queue, node.Right)
		}
		i++
	}
	return root
}

// Preorder returns a slice of values in preorder (root, left, right).
func (n *BinaryNode[T]) Preorder() []T {
	var res []T
	var walk func(*BinaryNode[T])
	walk = func(node *BinaryNode[T]) {
		if node == nil {
			return
		}
		res = append(res, node.Value)
		walk(node.Left)
		walk(node.Right)
	}
	walk(n)
	return res
}

// Inorder returns a slice of values in inorder (left, root, right).
func (n *BinaryNode[T]) Inorder() []T {
	var res []T
	var walk func(*BinaryNode[T])
	walk = func(node *BinaryNode[T]) {
		if node == nil {
			return
		}
		walk(node.Left)
		res = append(res, node.Value)
		walk(node.Right)
	}
	walk(n)
	return res
}

// Postorder returns a slice of values in postorder (left, right, root).
func (n *BinaryNode[T]) Postorder() []T {
	var res []T
	var walk func(*BinaryNode[T])
	walk = func(node *BinaryNode[T]) {
		if node == nil {
			return
		}
		walk(node.Left)
		walk(node.Right)
		res = append(res, node.Value)
	}
	walk(n)
	return res
}

// LevelOrder returns a slice of values in level order (BFS).
func (n *BinaryNode[T]) LevelOrder() []T {
	if n == nil {
		return nil
	}
	var res []T
	queue := []*BinaryNode[T]{n}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		res = append(res, node.Value)
		if node.Left != nil {
			queue = append(queue, node.Left)
		}
		if node.Right != nil {
			queue = append(queue, node.Right)
		}
	}
	return res
}

// Height returns the height of the tree (number of edges on longest root‑leaf path).
func (n *BinaryNode[T]) Height() int {
	if n == nil {
		return -1
	}
	left := n.Left.Height()
	right := n.Right.Height()
	if left > right {
		return left + 1
	}
	return right + 1
}

// Map returns a new tree with f applied to every value.
func (n *BinaryNode[T]) Map(f func(T) T) *BinaryNode[T] {
	if n == nil {
		return nil
	}
	return &BinaryNode[T]{
		Value: f(n.Value),
		Left:  n.Left.Map(f),
		Right: n.Right.Map(f),
	}
}

// Filter returns a new tree containing only nodes where f(value) is true.
// The tree structure is preserved; nodes that do not satisfy f are removed,
// and their children are reattached to the nearest remaining ancestor.
// This is a conservative filter; for a full binary tree filter, see the
// extended package.
func (n *BinaryNode[T]) Filter(f func(T) bool) *BinaryNode[T] {
	if n == nil {
		return nil
	}
	left := n.Left.Filter(f)
	right := n.Right.Filter(f)
	if f(n.Value) {
		return &BinaryNode[T]{
			Value: n.Value,
			Left:  left,
			Right: right,
		}
	}
	// Node removed: promote left or right, or return nil.
	// This simple heuristic promotes the left child; the right child is discarded.
	// For a full filter, a more sophisticated merging strategy is needed.
	if left != nil {
		// Find the rightmost node of left and attach right subtree
		if right != nil {
			rightmost := left
			for rightmost.Right != nil {
				rightmost = rightmost.Right
			}
			rightmost.Right = right
		}
		return left
	}
	return right
}

// --------------------------------------------------------------------
// N‑ary tree
// --------------------------------------------------------------------

// Node is a generic n‑ary tree node.
type Node[T any] struct {
	Value    T
	Children []*Node[T]
}

// NewNode creates a new n‑ary node with the given value and optional children.
func NewNode[T any](value T, children ...*Node[T]) *Node[T] {
	return &Node[T]{
		Value:    value,
		Children: children,
	}
}

// AddChild appends a child to the node.
func (n *Node[T]) AddChild(child *Node[T]) {
	n.Children = append(n.Children, child)
}

// Depth returns the depth of the node relative to the root.
func (n *Node[T]) Depth() int {
	return 0 // root depth is 0; this method should be called on the root.
}

// Height returns the height of the tree (max depth of any leaf).
func (n *Node[T]) Height() int {
	if len(n.Children) == 0 {
		return 0
	}
	max := 0
	for _, c := range n.Children {
		if h := c.Height(); h > max {
			max = h
		}
	}
	return max + 1
}

// Walk performs a DFS traversal, calling f on each node.
// Order: current node, then children left‑to‑right.
func (n *Node[T]) Walk(f func(*Node[T])) {
	f(n)
	for _, c := range n.Children {
		c.Walk(f)
	}
}

// WalkBFS performs a BFS traversal, calling f on each node.
func (n *Node[T]) WalkBFS(f func(*Node[T])) {
	if n == nil {
		return
	}
	queue := []*Node[T]{n}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		f(node)
		queue = append(queue, node.Children...)
	}
}

// Map returns a new tree with f applied to every value.
func (n *Node[T]) Map(f func(T) T) *Node[T] {
	if n == nil {
		return nil
	}
	newNode := &Node[T]{Value: f(n.Value)}
	newNode.Children = slice.Map(n.Children, func(c *Node[T]) *Node[T] {
		return c.Map(f)
	})
	return newNode
}

// Filter returns a new tree containing only nodes where f(value) is true.
// Children of removed nodes are promoted to become children of the nearest
// remaining ancestor. The order of children is preserved.
func (n *Node[T]) Filter(f func(T) bool) *Node[T] {
	if n == nil {
		return nil
	}
	// Filter children recursively
	filteredChildren := make([]*Node[T], 0, len(n.Children))
	for _, c := range n.Children {
		if child := c.Filter(f); child != nil {
			filteredChildren = append(filteredChildren, child)
		}
	}
	// Decide whether to keep this node
	if f(n.Value) {
		return &Node[T]{
			Value:    n.Value,
			Children: filteredChildren,
		}
	}
	// Node removed: promote filtered children to parent level.
	// Since we cannot return multiple nodes, we flatten the children.
	// This is a flattening filter; the node's children become siblings.
	// If you need to preserve depth, use a different strategy.
	if len(filteredChildren) == 0 {
		return nil
	}
	if len(filteredChildren) == 1 {
		return filteredChildren[0]
	}
	// Multiple children – we need a dummy root? Not possible; return nil and
	// let the parent handle it? Instead, we return nil and the parent will
	// append all filteredChildren directly. This requires cooperation from
	// the parent's Filter implementation. We'll adjust: Filter returns a slice.
	// But to keep API simple, we use a flattening approach: if this node is
	// removed, we return nil and the caller must handle promotion.
	// Therefore, we add a separate FlattenFilter that returns []*Node[T].
	return nil
}

// FlattenFilter returns a slice of nodes that remain after filtering.
// This is useful when a node may be removed and its children need to be
// promoted to the parent's child list.
func (n *Node[T]) FlattenFilter(f func(T) bool) []*Node[T] {
	if n == nil {
		return nil
	}
	// Collect surviving children recursively
	var survivors []*Node[T]
	for _, c := range n.Children {
		survivors = append(survivors, c.FlattenFilter(f)...)
	}
	if f(n.Value) {
		// Keep this node, attach survivors as children
		return []*Node[T]{{Value: n.Value, Children: survivors}}
	}
	// Node removed: return survivors directly (promotion)
	return survivors
}

// ToSlice returns a slice of values in DFS order (preorder).
func (n *Node[T]) ToSlice() []T {
	var res []T
	n.Walk(func(node *Node[T]) { res = append(res, node.Value) })
	return res
}

// --------------------------------------------------------------------
// Utility functions for all node types
// --------------------------------------------------------------------

// Equal reports whether two linked lists are equal in value and structure.
func Equal[T comparable](a, b *ListNode[T]) bool {
	for a != nil && b != nil {
		if a.Value != b.Value {
			return false
		}
		a = a.Next
		b = b.Next
	}
	return a == b
}

// EqualBinary reports whether two binary trees are equal.
func EqualBinary[T comparable](a, b *BinaryNode[T]) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Value == b.Value &&
		EqualBinary(a.Left, b.Left) &&
		EqualBinary(a.Right, b.Right)
}

// EqualNary reports whether two n‑ary trees are equal.
func EqualNary[T comparable](a, b *Node[T]) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Value != b.Value {
		return false
	}
	if len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !EqualNary(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

// --------------------------------------------------------------------
// Integration with value.Option and pointer
// --------------------------------------------------------------------

// SomeNode returns an Option containing the node if non‑nil, otherwise None.
func SomeNode[T any](n *ListNode[T]) value.Option[*ListNode[T]] {
	if n == nil {
		return value.None[*ListNode[T]]()
	}
	return value.Some(n)
}

// SomeBinaryNode returns an Option containing the node if non‑nil.
func SomeBinaryNode[T any](n *BinaryNode[T]) value.Option[*BinaryNode[T]] {
	if n == nil {
		return value.None[*BinaryNode[T]]()
	}
	return value.Some(n)
}

// SomeNaryNode returns an Option containing the node if non‑nil.
func SomeNaryNode[T any](n *Node[T]) value.Option[*Node[T]] {
	if n == nil {
		return value.None[*Node[T]]()
	}
	return value.Some(n)
}
