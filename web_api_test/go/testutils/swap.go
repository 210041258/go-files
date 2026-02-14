// Package swap provides generic functions for swapping values.
// It includes swapping two values, swapping through pointers,
// and swapping slice elements.
package testutils

// Values returns its two arguments swapped.
// Example: a, b := swap.Values(1, 2) // a=2, b=1
func Values[T any](a, b T) (T, T) {
	return b, a
}

// Pointers swaps the values stored at the given pointers.
// Example:
//   x, y := 1, 2
//   swap.Pointers(&x, &y)
//   // x=2, y=1
func Pointers[T any](a, b *T) {
	*a, *b = *b, *a
}

// Slice swaps the elements at indices i and j in the slice.
// It panics if i or j are out of range.
// Example:
//   s := []int{1,2,3}
//   swap.Slice(s, 0, 2) // s = [3,2,1]
func Slice[T any](s []T, i, j int) {
	s[i], s[j] = s[j], s[i]
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     a, b := swap.Values("hello", "world")
//     fmt.Println(a, b) // world hello
//
//     x, y := 10, 20
//     swap.Pointers(&x, &y)
//     fmt.Println(x, y) // 20 10
//
//     slice := []int{5,6,7,8}
//     swap.Slice(slice, 1, 3)
//     fmt.Println(slice) // [5,8,7,6]
// }