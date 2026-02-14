// Package data provides utilities for manipulating unstructured data
// (e.g., JSON objects, maps, slices). It includes functions for deep
// copying, merging, traversing nested structures, and converting between
// nested and flattened representations.
package testutils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Deep copy
// ----------------------------------------------------------------------

// DeepCopy recursively copies a value of any type.
// It handles maps, slices, and basic types.
func DeepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Map:
		// Create a new map of the same type.
		newMap := reflect.MakeMap(val.Type())
		for _, key := range val.MapKeys() {
			newMap.SetMapIndex(key, reflect.ValueOf(DeepCopy(val.MapIndex(key).Interface())))
		}
		return newMap.Interface()
	case reflect.Slice:
		// Create a new slice of the same type and length.
		newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Len())
		for i := 0; i < val.Len(); i++ {
			newSlice.Index(i).Set(reflect.ValueOf(DeepCopy(val.Index(i).Interface())))
		}
		return newSlice.Interface()
	default:
		// For basic types (int, string, etc.), just return the value.
		return v
	}
}

// ----------------------------------------------------------------------
// Deep merge (map only)
// ----------------------------------------------------------------------

// Merge combines two maps recursively. For keys that exist in both,
// if both values are maps, they are merged recursively; otherwise,
// the source value overwrites the destination value.
// It returns a new map; the original maps are not modified.
func Merge(dst, src map[string]interface{}) map[string]interface{} {
	result := DeepCopy(dst).(map[string]interface{})
	for sk, sv := range src {
		if dv, ok := result[sk]; ok {
			// If both values are maps, merge recursively.
			if dvm, dok := dv.(map[string]interface{}); dok {
				if svm, sok := sv.(map[string]interface{}); sok {
					result[sk] = Merge(dvm, svm)
					continue
				}
			}
		}
		// Otherwise, overwrite with source value (deep copied).
		result[sk] = DeepCopy(sv)
	}
	return result
}

// ----------------------------------------------------------------------
// Path access (e.g., "a.b.c")
// ----------------------------------------------------------------------

// GetByPath retrieves a value from a nested map using a dot‑separated path.
// If a path component is an integer in brackets (e.g., "items[0]"), it is
// interpreted as a slice index. Returns an error if the path is invalid.
func GetByPath(m map[string]interface{}, path string) (interface{}, error) {
	if path == "" {
		return m, nil
	}
	parts := strings.Split(path, ".")
	var current interface{} = m
	for _, part := range parts {
		// Check for array index: e.g., "items[0]"
		if idxStart := strings.Index(part, "["); idxStart != -1 && strings.HasSuffix(part, "]") {
			mapPart := part[:idxStart]
			idxStr := part[idxStart+1 : len(part)-1]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index %q in path %q", idxStr, path)
			}
			// First, navigate the map part.
			if mapPart != "" {
				switch v := current.(type) {
				case map[string]interface{}:
					var ok bool
					current, ok = v[mapPart]
					if !ok {
						return nil, fmt.Errorf("key %q not found", mapPart)
					}
				default:
					return nil, fmt.Errorf("expected map at %q, got %T", mapPart, current)
				}
			}
			// Then navigate the slice.
			switch v := current.(type) {
			case []interface{}:
				if idx < 0 || idx >= len(v) {
					return nil, fmt.Errorf("index %d out of range (length %d)", idx, len(v))
				}
				current = v[idx]
			default:
				return nil, fmt.Errorf("expected slice at %q, got %T", part, current)
			}
		} else {
			// Normal map key.
			switch v := current.(type) {
			case map[string]interface{}:
				var ok bool
				current, ok = v[part]
				if !ok {
					return nil, fmt.Errorf("key %q not found", part)
				}
			default:
				return nil, fmt.Errorf("expected map at %q, got %T", part, current)
			}
		}
	}
	return current, nil
}

// SetByPath sets a value in a nested map using a dot‑separated path.
// Intermediate maps are created if they do not exist. Slice indices must
// already exist; if an index is out of range, an error is returned.
func SetByPath(m map[string]interface{}, path string, value interface{}) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	parts := strings.Split(path, ".")
	var current interface{} = m
	for i, part := range parts {
		isLast := i == len(parts)-1
		// Check for array index.
		if idxStart := strings.Index(part, "["); idxStart != -1 && strings.HasSuffix(part, "]") {
			mapPart := part[:idxStart]
			idxStr := part[idxStart+1 : len(part)-1]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return fmt.Errorf("invalid array index %q", idxStr)
			}
			// Navigate the map part if present.
			if mapPart != "" {
				switch v := current.(type) {
				case map[string]interface{}:
					if next, ok := v[mapPart]; ok {
						current = next
					} else if !isLast {
						// Create intermediate map.
						newMap := make(map[string]interface{})
						v[mapPart] = newMap
						current = newMap
					} else {
						// Last part with mapPart and index? Not typical.
						return fmt.Errorf("cannot set value at %q: map key missing", mapPart)
					}
				default:
					return fmt.Errorf("expected map at %q, got %T", mapPart, current)
				}
			}
			// Now navigate the slice.
			switch v := current.(type) {
			case []interface{}:
				if idx < 0 || idx >= len(v) {
					return fmt.Errorf("index %d out of range (length %d)", idx, len(v))
				}
				if isLast {
					v[idx] = value
				} else {
					current = v[idx]
				}
			default:
				return fmt.Errorf("expected slice at %q, got %T", part, current)
			}
		} else {
			// Normal map key.
			switch v := current.(type) {
			case map[string]interface{}:
				if isLast {
					v[part] = value
				} else {
					if next, ok := v[part]; ok {
						current = next
					} else {
						// Create intermediate map.
						newMap := make(map[string]interface{})
						v[part] = newMap
						current = newMap
					}
				}
			default:
				return fmt.Errorf("expected map at %q, got %T", part, current)
			}
		}
	}
	return nil
}

// ----------------------------------------------------------------------
// Flatten and unflatten
// ----------------------------------------------------------------------

// Flatten converts a nested map into a single‑level map with dot‑separated keys.
// Example: {"a": {"b": 1}} -> {"a.b": 1}
func Flatten(m map[string]interface{}) map[string]interface{} {
	return flattenHelper(m, "")
}

func flattenHelper(m map[string]interface{}, parentKey string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		fullKey := k
		if parentKey != "" {
			fullKey = parentKey + "." + k
		}
		switch child := v.(type) {
		case map[string]interface{}:
			nested := flattenHelper(child, fullKey)
			for nk, nv := range nested {
				result[nk] = nv
			}
		case []interface{}:
			// For slices, we flatten each element with index notation.
			for i, elem := range child {
				elemKey := fmt.Sprintf("%s[%d]", fullKey, i)
				if elemMap, ok := elem.(map[string]interface{}); ok {
					nested := flattenHelper(elemMap, elemKey)
					for nk, nv := range nested {
						result[nk] = nv
					}
				} else {
					result[elemKey] = elem
				}
			}
		default:
			result[fullKey] = v
		}
	}
	return result
}

// Unflatten converts a flat map with dot‑separated keys back into a nested map.
// Example: {"a.b": 1} -> {"a": {"b": 1}}
func Unflatten(flat map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range flat {
		keys := strings.Split(k, ".")
		current := result
		for i, key := range keys {
			isLast := i == len(keys)-1
			// Check for array index in key, e.g., "items[0]"
			if idxStart := strings.Index(key, "["); idxStart != -1 && strings.HasSuffix(key, "]") {
				mapPart := key[:idxStart]
				idxStr := key[idxStart+1 : len(key)-1]
				idx, err := strconv.Atoi(idxStr)
				if err != nil {
					// If invalid index, treat as normal key? For simplicity, we skip.
					continue
				}
				// Ensure the map part exists.
				if mapPart != "" {
					if _, ok := current[mapPart]; !ok {
						current[mapPart] = make([]interface{}, 0)
					}
					// The value at mapPart must be a slice.
					switch arr := current[mapPart].(type) {
					case []interface{}:
						// Ensure slice is long enough.
						for len(arr) <= idx {
							arr = append(arr, nil)
						}
						if isLast {
							arr[idx] = v
						} else {
							if arr[idx] == nil {
								// Create intermediate map.
								arr[idx] = make(map[string]interface{})
							}
							current = arr[idx].(map[string]interface{})
						}
						current[mapPart] = arr
					default:
						// Overwrite if not a slice? For simplicity, create new slice.
						newArr := make([]interface{}, idx+1)
						newArr[idx] = v
						current[mapPart] = newArr
					}
				} else {
					// No map part, so key is directly an array at current level.
					// This shouldn't happen for top‑level, but handle anyway.
				}
			} else {
				// Normal map key.
				if isLast {
					current[key] = v
				} else {
					if _, ok := current[key]; !ok {
						current[key] = make(map[string]interface{})
					}
					current = current[key].(map[string]interface{})
				}
			}
		}
	}
	return result
}

// ----------------------------------------------------------------------
// Recursive key transformation
// ----------------------------------------------------------------------

// TransformKeys recursively transforms all keys in maps using the provided function.
// It also processes slices of maps. Other values are left unchanged.
func TransformKeys(v interface{}, fn func(string) string) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		newMap := make(map[string]interface{})
		for k, v := range val {
			newKey := fn(k)
			newMap[newKey] = TransformKeys(v, fn)
		}
		return newMap
	case []interface{}:
		newSlice := make([]interface{}, len(val))
		for i, elem := range val {
			newSlice[i] = TransformKeys(elem, fn)
		}
		return newSlice
	default:
		return v
	}
}

// ----------------------------------------------------------------------
// JSON round‑trip helpers
// ----------------------------------------------------------------------

// FromJSON parses a JSON string into a generic Go value (map[string]interface{}
// or []interface{}). It is a convenience wrapper around json.Unmarshal.
func FromJSON(s string) (interface{}, error) {
	var v interface{}
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

// ToJSON converts a generic Go value to a JSON string with indentation.
func ToJSON(v interface{}, indent bool) (string, error) {
	var b []byte
	var err error
	if indent {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	return string(b), err
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     m := map[string]interface{}{
//         "a": 1,
//         "b": map[string]interface{}{
//             "c": 2,
//             "d": []interface{}{3, 4},
//         },
//     }
//
//     // Deep copy
//     copy := data.DeepCopy(m)
//
//     // Merge
//     merged := data.Merge(m, map[string]interface{}{"a": 10, "b": map[string]interface{}{"e": 5}})
//
//     // Get by path
//     val, _ := data.GetByPath(m, "b.d[1]") // 4
//
//     // Set by path
//     data.SetByPath(m, "b.d[0]", 99)
//
//     // Flatten
//     flat := data.Flatten(m)
//     // {"a":1,"b.c":2,"b.d[0]":99,"b.d[1]":4}
//
//     // Transform keys to uppercase
//     upper := data.TransformKeys(m, strings.ToUpper)
// }
