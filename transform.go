// Package transform provides utilities for transforming data between
// different formats, encodings, and structures. It includes common
// string case conversions, encoding/decoding, and struct/map conversions.
package testutils

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"unicode"
)

// ----------------------------------------------------------------------
// String case transformations
// ----------------------------------------------------------------------

// ToSnakeCase converts a string to snake_case.
// Example: "UserID" -> "user_id", "HTTPRequest" -> "http_request".
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 && !unicode.IsUpper(rune(s[i-1])) {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ToCamelCase converts a snake_case or kebab-case string to camelCase.
// Example: "user_id" -> "userId", "http-request" -> "httpRequest".
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	upperNext := false
	for i, r := range s {
		if r == '_' || r == '-' {
			upperNext = true
			continue
		}
		if upperNext || i == 0 {
			result.WriteRune(unicode.ToUpper(r))
			upperNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ToPascalCase converts a snake_case or kebab-case string to PascalCase.
// Example: "user_id" -> "UserId", "http-request" -> "HttpRequest".
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}
	camel := ToCamelCase(s)
	return strings.ToUpper(camel[:1]) + camel[1:]
}

// ToKebabCase converts a string to kebab-case.
// Example: "UserID" -> "user-id", "HTTPRequest" -> "http-request".
func ToKebabCase(s string) string {
	return strings.ReplaceAll(ToSnakeCase(s), "_", "-")
}

// ----------------------------------------------------------------------
// Encoding / Decoding
// ----------------------------------------------------------------------

// Base64Encode encodes bytes to a standard base64 string.
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode decodes a standard base64 string.
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// Base64URLEncode encodes bytes to a URL‑safe base64 string (without padding).
func Base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// Base64URLDecode decodes a URL‑safe base64 string (without padding).
func Base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// HexEncode encodes bytes to a hexadecimal string.
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// HexDecode decodes a hexadecimal string.
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// URLEncode percent‑encodes a string according to RFC 3986.
func URLEncode(s string) string {
	return url.QueryEscape(s)
}

// URLDecode percent‑decodes a string.
func URLDecode(s string) (string, error) {
	return url.QueryUnescape(s)
}

// ----------------------------------------------------------------------
// JSON transformations
// ----------------------------------------------------------------------

// JSONToMap parses a JSON string into a map[string]interface{}.
func JSONToMap(s string) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal([]byte(s), &m)
	return m, err
}

// MapToJSON converts a map to a JSON string with optional indentation.
func MapToJSON(m map[string]interface{}, indent bool) (string, error) {
	var b []byte
	var err error
	if indent {
		b, err = json.MarshalIndent(m, "", "  ")
	} else {
		b, err = json.Marshal(m)
	}
	return string(b), err
}

// ----------------------------------------------------------------------
// Struct <-> map conversions (using reflection)
// ----------------------------------------------------------------------

// StructToMap converts a struct to a map[string]interface{} using field names as keys.
// Only exported fields are included. If a field has a `json` tag, that tag name is used.
func StructToMap(v interface{}) (map[string]interface{}, error) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %T", v)
	}
	typ := val.Type()
	result := make(map[string]interface{})
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.CanInterface() {
			continue // unexported
		}
		fieldType := typ.Field(i)
		// Use json tag if present.
		tag := fieldType.Tag.Get("json")
		if tag != "" {
			// Ignore options like ",omitempty"
			if idx := strings.Index(tag, ","); idx != -1 {
				tag = tag[:idx]
			}
			if tag == "-" {
				continue
			}
		} else {
			tag = fieldType.Name
		}
		result[tag] = field.Interface()
	}
	return result, nil
}

// MapToStruct populates a struct from a map using field names or json tags.
// The struct must be passed as a pointer.
func MapToStruct(m map[string]interface{}, v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return fmt.Errorf("expected non‑nil pointer to struct")
	}
	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct pointer, got %T", v)
	}
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		if !fieldVal.CanSet() {
			continue // unexported
		}
		fieldType := typ.Field(i)
		// Determine key name.
		key := fieldType.Name
		if tag := fieldType.Tag.Get("json"); tag != "" {
			if idx := strings.Index(tag, ","); idx != -1 {
				tag = tag[:idx]
			}
			if tag != "-" {
				key = tag
			} else {
				continue
			}
		}
		if mapVal, ok := m[key]; ok {
			// Simple assignment if types match (or can be converted).
			mapReflect := reflect.ValueOf(mapVal)
			if mapReflect.Type().AssignableTo(fieldVal.Type()) {
				fieldVal.Set(mapReflect)
			} else if mapReflect.Type().ConvertibleTo(fieldVal.Type()) {
				fieldVal.Set(mapReflect.Convert(fieldVal.Type()))
			}
			// Otherwise ignore (could return error, but for simplicity skip).
		}
	}
	return nil
}

// ----------------------------------------------------------------------
// Transformer interface and composition
// ----------------------------------------------------------------------

// Transformer transforms a value of type T into another value of type U.
type Transformer[T, U any] interface {
	Transform(T) (U, error)
}

// TransformerFunc is a function that implements Transformer.
type TransformerFunc[T, U any] func(T) (U, error)

func (f TransformerFunc[T, U]) Transform(t T) (U, error) {
	return f(t)
}

// Compose returns a Transformer that applies f then g.
func Compose[T, U, V any](f Transformer[T, U], g Transformer[U, V]) Transformer[T, V] {
	return TransformerFunc[T, V](func(t T) (V, error) {
		u, err := f.Transform(t)
		if err != nil {
			var zero V
			return zero, err
		}
		return g.Transform(u)
	})
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Case conversion
//     fmt.Println(transform.ToSnakeCase("UserID"))      // "user_id"
//     fmt.Println(transform.ToCamelCase("user_id"))    // "userId"
//
//     // Encoding
//     enc := transform.Base64Encode([]byte("hello"))
//     dec, _ := transform.Base64Decode(enc)
//
//     // Struct to map
//     type User struct {
//         Name string `json:"name"`
//         Age  int    `json:"age"`
//     }
//     u := User{Name: "Alice", Age: 30}
//     m, _ := transform.StructToMap(u)
//     fmt.Println(m) // map[age:30 name:Alice]
//
//     // Map to struct
//     var u2 User
//     transform.MapToStruct(m, &u2)
// }