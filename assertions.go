package testutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// AssertStatusCode verifies HTTP response status code.
// It also captures and prints the response body on failure to aid debugging.
func AssertStatusCode(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		// Read body for error reporting, then replace it so the caller can still read it if needed (though usually Close follows)
		bodyBytes, _ := io.ReadAll(response.Body)

		// Attempt to restore body for caller (rarely used, but good practice)
		response.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		t.Errorf("Expected status %d, received %d\nResponse Body: %s",
			expected, response.StatusCode, string(bodyBytes))
	}
}

// AssertContentType verifies response content type header.
func AssertContentType(t *testing.T, response *http.Response, expected string) {
	t.Helper()
	contentType := response.Header.Get("Content-Type")
	if !strings.Contains(contentType, expected) {
		t.Errorf("Expected Content-Type containing %s, received %s", expected, contentType)
	}
}

// AssertFieldExists verifies a field exists in a map.
func AssertFieldExists(t *testing.T, data map[string]interface{}, field string) {
	t.Helper()
	if _, exists := data[field]; !exists {
		// Pretty print keys to help debugging
		keys := getMapKeys(data)
		t.Errorf("Required field '%s' is missing. Available keys: %v", field, keys)
	}
}

// AssertFieldEquals verifies field value matches expected value.
// Improved: Uses reflect.DeepEqual to handle type mismatches (e.g., int vs float64 from JSON).
func AssertFieldEquals(t *testing.T, data map[string]interface{}, field string, expected interface{}) {
	t.Helper()
	actual, exists := data[field]
	if !exists {
		t.Errorf("Field '%s' does not exist", field)
		return
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Field '%s' mismatch:\n  Expected: %v (%T)\n  Received: %v (%T)",
			field, expected, expected, actual, actual)
	}
}

// AssertFieldEqualsType verifies field value matches expected value and strictly checks types.
// This is useful when you specifically require an Integer and not a Float.
func AssertFieldEqualsType(t *testing.T, data map[string]interface{}, field string, expected interface{}) {
	t.Helper()
	actual, exists := data[field]
	if !exists {
		t.Errorf("Field '%s' does not exist", field)
		return
	}

	if actual != expected {
		t.Errorf("Field '%s' mismatch:\n  Expected: %v\n  Received: %v", field, expected, actual)
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Field '%s' type mismatch:\n  Expected: %T\n  Received: %T",
			field, expected, actual)
	}
}

// AssertJSONFieldPath checks a value at a specific dot-notation path (e.g., "user.address.city").
// This avoids unmarshalling the whole body into a map manually in the test.
func AssertJSONFieldPath(t *testing.T, responseBody []byte, path string, expected interface{}) {
	t.Helper()
	var data interface{}
	if err := json.Unmarshal(responseBody, &data); err != nil {
		t.Fatalf("Failed to unmarshal JSON response: %v", err)
	}

	result := getNestedValue(data, strings.Split(path, "."))
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Field at path '%s' mismatch:\n  Expected: %v\n  Received: %v",
			path, expected, result)
	}
}

// getNestedValue retrieves a value from a nested map/slice structure using a key path.
func getNestedValue(data interface{}, keys []string) interface{} {
	if len(keys) == 0 {
		return data
	}

	current := data
	key := keys[0]
	remaining := keys[1:]

	// Handle Map access
	if m, ok := current.(map[string]interface{}); ok {
		return getNestedValue(m[key], remaining)
	}

	// Handle Slice access (numeric string keys)
	if s, ok := current.([]interface{}); ok {
		index := 0
		if _, err := fmt.Sscanf(key, "%d", &index); err == nil {
			if index >= 0 && index < len(s) {
				return getNestedValue(s[index], remaining)
			}
		}
	}

	return nil
}

// getMapKeys returns a slice of keys from a map for debugging purposes.
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// IsValidEmailFormat performs basic email format validation.
// Improved: Checks for valid positions of "@" and "." to avoid accepting "a@b" or "a..b".
func IsValidEmailFormat(email string) bool {
	if len(email) < 6 { // a@b.co is minimum reasonable length
		return false
	}

	atIndex := strings.LastIndex(email, "@")
	dotIndex := strings.LastIndex(email, ".")

	// Must contain @ and .
	if atIndex == -1 || dotIndex == -1 {
		return false
	}

	// @ must come before the last .
	if atIndex > dotIndex {
		return false
	}

	// @ must not be first char
	if atIndex == 0 {
		return false
	}

	// . must not be last char or immediately after @
	if dotIndex == len(email)-1 || dotIndex == atIndex+1 {
		return false
	}

	return true
}
