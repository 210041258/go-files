// Package maputil provides generic utilities for working with Go maps.
// It includes functions for common operations like extracting keys/values,
// merging, filtering, and transforming maps.
package testutils

import (
	"reflect"
)

// Keys returns a slice containing all keys in the map.
// The order is not guaranteed.
func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns a slice containing all values in the map.
// The order is not guaranteed.
func Values[K comparable, V any](m map[K]V) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}

// Merge combines multiple maps into one. Later maps overwrite earlier ones.
// Returns a new map; the input maps are not modified.
func Merge[K comparable, V any](maps ...map[K]V) map[K]V {
	result := make(map[K]V)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// Copy creates a shallow copy of the map.
func Copy[K comparable, V any](m map[K]V) map[K]V {
	cpy := make(map[K]V, len(m))
	for k, v := range m {
		cpy[k] = v
	}
	return cpy
}

// Filter returns a new map containing only entries where the predicate returns true.
func Filter[K comparable, V any](m map[K]V, pred func(K, V) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		if pred(k, v) {
			result[k] = v
		}
	}
	return result
}

// MapValues returns a new map where each value is transformed by the provided function.
// Keys remain unchanged.
func MapValues[K comparable, V any, U any](m map[K]V, f func(V) U) map[K]U {
	result := make(map[K]U, len(m))
	for k, v := range m {
		result[k] = f(v)
	}
	return result
}

// MapKeys returns a new map where each key is transformed by the provided function.
// Values remain unchanged. If the function produces duplicate keys, later values overwrite earlier ones.
func MapKeys[K comparable, V any, L comparable](m map[K]V, f func(K) L) map[L]V {
	result := make(map[L]V, len(m))
	for k, v := range m {
		result[f(k)] = v
	}
	return result
}

// HasKey checks whether the map contains the given key.
func HasKey[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

// GetOrDefault returns the value for the key if it exists, otherwise returns the default value.
func GetOrDefault[K comparable, V any](m map[K]V, key K, defaultValue V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultValue
}

// Pick returns a new map containing only the specified keys.
// If a key does not exist, it is omitted.
func Pick[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	result := make(map[K]V)
	for _, k := range keys {
		if v, ok := m[k]; ok {
			result[k] = v
		}
	}
	return result
}

// Omit returns a new map with the specified keys removed.
func Omit[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	result := Copy(m)
	for _, k := range keys {
		delete(result, k)
	}
	return result
}

// Invert swaps keys and values. Values must be comparable.
// If multiple keys have the same value, one of them will be kept (the last encountered).
func Invert[K comparable, V comparable](m map[K]V) map[V]K {
	result := make(map[V]K, len(m))
	for k, v := range m {
		result[v] = k
	}
	return result
}

// Equal reports whether two maps contain the same key/value pairs.
// It uses reflect.DeepEqual to compare values. For performance‑sensitive code,
// consider a custom comparator.
func Equal[K comparable, V any](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(va, vb) {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     m := map[string]int{"a": 1, "b": 2, "c": 3}
//     fmt.Println(maputil.Keys(m))   // [a b c] (order may vary)
//     fmt.Println(maputil.Values(m)) // [1 2 3]
//
//     filtered := maputil.Filter(m, func(k string, v int) bool { return v > 1 })
//     fmt.Println(filtered) // map[b:2 c:3]
//
//     doubled := maputil.MapValues(m, func(v int) int { return v * 2 })
//     fmt.Println(doubled) // map[a:2 b:4 c:6]
//
//     picked := maputil.Pick(m, "a", "c")
//     fmt.Println(picked) // map[a:1 c:3]
// }