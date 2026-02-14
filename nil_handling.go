package testutils

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"
)

// ==========================================
// 1. Basic Nil Pointers, Slices, Maps, Interfaces
// ==========================================

func BasicNilExamples() {
	// Pointers: nil represents the absence of a memory address
	var p *int
	fmt.Println("Pointer:", p == nil) // true

	// Slices: nil is the zero value, useful to distinguish "empty" vs "not set"
	var s []int
	fmt.Println("Slice:", s == nil, len(s), cap(s)) // true, 0, 0

	// Maps: nil maps are readable but not writable
	var m map[string]string
	// m["key"] = "val" // Panic: assignment to entry in nil map
	fmt.Println("Map:", m == nil) // true

	// Interfaces: A nil interface value holds neither type nor value.
	var i interface{}
	fmt.Println("Interface:", i == nil) // true
}

// ==========================================
// 2. Struct Fields with Nil (Optional Values)
// ==========================================

type UserProfile struct {
	ID       *int    // Optional ID (0 is valid, but nil means unknown/unset)
	Nickname *string // Pointer to string allows distinguishing "" (empty) vs nil (unset)
	Bio      *string
}

func StructNilExample() {
	// Scenario: Creating a profile where ID is known, but Nickname isn't provided.
	id := 123
	profile := UserProfile{
		ID:       &id,
		Nickname: nil, // Explicitly not set
		Bio:      nil,
	}

	if profile.Nickname == nil {
		fmt.Println("Nickname is not set")
	} else {
		fmt.Println("Nickname:", *profile.Nickname)
	}
}

// ==========================================
// 3. JSON: omitempty, Explicit Null, Pointers vs Zero
// ==========================================

type JSONPayload struct {
	Name     string  `json:"name"`
	Age      int     `json:"age"`
	Email    *string `json:"email,omitempty"` // Omits if nil OR empty string
	Phone    *string `json:"phone"`           // Includes null explicitly if nil
	IsActive bool    `json:"is_active,omitempty"`
}

func JSONMarshalingExample() {
	email := "test@example.com"
	payload := JSONPayload{
		Name:  "John",
		Age:   30,
		Email: &email, // Present
		// Phone is nil (will show as "phone": null in JSON)
	}

	data, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println("JSON Output:\n", string(data))
	/*
	   Output:
	   {
	     "name": "John",
	     "age": 30,
	     "email": "test@example.com",
	     "phone": null
	   }
	*/
}

// ==========================================
// 4. Database: sql.NullString, sql.NullTime, Custom NullInt64
// ==========================================

func DatabaseScanExample() {
	// 1. Using Standard Library sql.Null*
	var nullStr sql.NullString
	nullStr.String = "Hello"
	nullStr.Valid = true

	if nullStr.Valid {
		fmt.Println("DB String:", nullStr.String)
	}

	var nullTime sql.NullTime
	nullTime.Valid = false // Represents SQL NULL

	// 2. Custom NullInt64 (useful for JSON API compatibility)
	// See definition of NullInt64 below
	val := NullInt64{Int64: 42, Valid: true}

	// Simulating a Scan
	val.Scan("100")
	fmt.Println("Custom NullInt64:", val.Int64, val.Valid)
}

// NullInt64 implements sql.Scanner and driver.Valuer for Nullable Integers
// This solves the issue where sql.NullInt64 doesn't serialize to JSON as `null`.
type NullInt64 struct {
	Int64 int64
	Valid bool
}

// Scan implements the sql.Scanner interface.
func (n *NullInt64) Scan(value interface{}) error {
	if value == nil {
		n.Int64, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	// Handle different DB driver return types
	switch v := value.(type) {
	case []byte:
		_, err := fmt.Sscanf(string(v), "%d", &n.Int64)
		return err
	case int64:
		n.Int64 = v
		return nil
	default:
		return errors.New("unsupported type for NullInt64")
	}
}

// Value implements the driver.Valuer interface.
func (n NullInt64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Int64, nil
}

// MarshalJSON for custom JSON output (null vs 0)
func (n NullInt64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int64)
}

// UnmarshalJSON allows reading `null` from JSON
// Improved: Returns error if data is invalid, preventing silent failures.
func (n *NullInt64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("cannot unmarshal %s into NullInt64", string(data))
	}
	n.Int64 = v
	n.Valid = true
	return nil
}

// ==========================================
// 5. Environment Variables: Pointers vs Empty
// ==========================================

// GetEnv retrieves an environment variable.
// Returns (value, true) if set, even if empty "".
// Returns (nil, false) if the variable does not exist.
func GetEnv(key string) (*string, bool) {
	val, exists := os.LookupEnv(key)
	if !exists {
		return nil, false
	}
	return &val, true
}

func EnvExample() {
	// Scenario: Allow user to set a variable to empty string intentionally
	os.Setenv("MY_VAR", "") // User explicitly set it to empty

	val, exists := GetEnv("MY_VAR")
	if exists && val != nil {
		fmt.Printf("Variable is set, value is: '%s'\n", *val) // Prints empty string
	} else if exists && val != nil && *val == "" {
		fmt.Println("Variable is explicitly empty")
	}

	// Scenario 2: Variable not set at all
	_, exists2 := GetEnv("NON_EXISTENT_VAR")
	if !exists2 {
		fmt.Println("Variable is not set in environment")
	}
}

// ==========================================
// 6. Interface Pitfalls: Typed Nil vs Pure Nil
// ==========================================

type Errorer interface {
	Error() string
}

// MyError implements Errorer
type MyError struct {
	Msg string
}

func (e *MyError) Error() string {
	return e.Msg
}

func ReturnError() Errorer {
	// Pitfall: Returning a typed nil pointer.
	// Even though 'ret' is nil, the return type is Errorer, and it holds (*MyError, nil).
	// An interface containing a non-nil type is not nil.
	var ret *MyError
	if true {
		return ret // Returns (<*MyError>, nil). Interface is NOT nil.
	}
	return nil // Returns (nil, nil). Interface IS nil.
}

func InterfaceNilExample() {
	var err Errorer = ReturnError()

	// This check will FAIL even though the underlying pointer is nil!
	if err != nil {
		fmt.Println("Pitfall: Interface is not nil, but underlying value might be.")
		// Debugging:
		fmt.Printf("Type: %T, Value: %v\n", err, err)
		// Output: Type: *common.MyError, Value: <nil>
	}
}

// IsActuallyNil safely checks if an interface is truly nil.
// Improved: Checks if the value is reflectable before calling IsNil to prevent panics.
func IsActuallyNil(i interface{}) bool {
	if i == nil {
		return true
	}

	// Get the reflect value. If it's invalid or cannot be set, it's likely not nil in a way we care about,
	// but checking IsNil on unreflectable types (like unpointed structs) causes panics.
	rv := reflect.ValueOf(i)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Func, reflect.Interface, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

// ==========================================
// 7. Custom Null Types with JSON Marshaling (Deep Dive)
// ==========================================

// NullTime is a wrapper around time.Time that supports null for JSON and SQL.
type NullTime struct {
	Time  time.Time
	Valid bool
}

// NewNullTime creates a valid NullTime
func NewNullTime(t time.Time) NullTime {
	return NullTime{Time: t, Valid: true}
}

// Scan implements sql.Scanner
// Improved: Handles both []byte (standard) and direct time.Time assignments safely.
func (nt *NullTime) Scan(value interface{}) error {
	if value == nil {
		nt.Time, nt.Valid = time.Time{}, false
		return nil
	}
	nt.Valid = true
	// Drivers usually return []byte, but sometimes time.Time directly.
	switch v := value.(type) {
	case time.Time:
		nt.Time = v
		return nil
	case []byte:
		// We assume the byte slice is a valid time format for the driver.
		// Using UnmarshalBinary is a safer bet for standard sql drivers, but we
		// catch panic if driver sends garbage.
		defer func() {
			if r := recover(); r != nil {
				nt.Valid = false
			}
		}()
		return nt.Time.UnmarshalBinary(v)
	default:
		return fmt.Errorf("cannot scan %T into NullTime", value)
	}
}

// Value implements driver.Valuer
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

// MarshalJSON implements json.Marshaler
func (nt NullTime) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return []byte("null"), nil
	}
	return nt.Time.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler
// Improved: Returns error if the data is invalid JSON, rather than silently failing.
func (nt *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		nt.Valid = false
		return nil
	}

	// If the data is not null, it must be a valid time string
	if err := nt.Time.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("invalid time format for NullTime: %w", err)
	}

	nt.Valid = true
	return nil
}
