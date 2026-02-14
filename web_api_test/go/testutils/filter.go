// Package testutils provides utilities for testing, including
// filtering operations on slices, maps, and strings.
package testutils

// FilterSlice returns a new slice containing only the elements for which
// the predicate function returns true.
// The original slice is not modified.
func FilterSlice[T any](slice []T, pred func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if pred(v) {
			result = append(result, v)
		}
	}
	return result
}

// FilterMap returns a new map containing only the key‑value pairs for which
// the predicate function returns true.
// The original map is not modified.
func FilterMap[K comparable, V any](m map[K]V, pred func(K, V) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		if pred(k, v) {
			result[k] = v
		}
	}
	return result
}

// FilterString returns a new string containing only the runes for which
// the keep function returns true.
// It processes the string as Unicode code points.
func FilterString(s string, keep func(rune) bool) string {
	runes := []rune(s)
	result := make([]rune, 0, len(runes))
	for _, r := range runes {
		if keep(r) {
			result = append(result, r)
		}
	}
	return string(result)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     nums := []int{1,2,3,4,5}
//     evens := testutils.FilterSlice(nums, func(n int) bool { return n%2==0 })
//     fmt.Println(evens) // [2 4]
//
//     m := map[string]int{"a":1, "b":2, "c":3}
//     filtered := testutils.FilterMap(m, func(k string, v int) bool { return v>1 })
//     fmt.Println(filtered) // map[b:2 c:3]
//
//     s := "hello 123 world"
//     digits := testutils.FilterString(s, func(r rune) bool { return r>='0' && r<='9' })
//     fmt.Println(digits) // "123"
// }