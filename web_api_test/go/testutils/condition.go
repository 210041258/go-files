// Package condition provides a framework for defining and evaluating
// logical conditions against a context. It supports basic comparisons,
// logical combinators, and a builder API for constructing complex conditions.
package testutils

import (
	"fmt"
	"reflect"
	"strings"
)

// ----------------------------------------------------------------------
// Condition interface
// ----------------------------------------------------------------------

// Condition is the interface that all conditions implement.
// Evaluate returns true if the condition holds for the given context.
type Condition interface {
	Evaluate(ctx map[string]interface{}) (bool, error)
}

// ----------------------------------------------------------------------
// Basic comparison conditions
// ----------------------------------------------------------------------

// Equals checks if the value at key equals the expected value.
type Equals struct {
	Key      string
	Expected interface{}
}

func (c Equals) Evaluate(ctx map[string]interface{}) (bool, error) {
	val, ok := ctx[c.Key]
	if !ok {
		return false, fmt.Errorf("key %q not found in context", c.Key)
	}
	return reflect.DeepEqual(val, c.Expected), nil
}

// NotEquals checks if the value at key does not equal the expected value.
type NotEquals struct {
	Key      string
	Expected interface{}
}

func (c NotEquals) Evaluate(ctx map[string]interface{}) (bool, error) {
	val, ok := ctx[c.Key]
	if !ok {
		return false, fmt.Errorf("key %q not found in context", c.Key)
	}
	return !reflect.DeepEqual(val, c.Expected), nil
}

// GreaterThan checks if the value at key (as a float64) is > threshold.
// It attempts numeric conversion if the stored value is not float64.
type GreaterThan struct {
	Key       string
	Threshold float64
}

func (c GreaterThan) Evaluate(ctx map[string]interface{}) (bool, error) {
	val, ok := ctx[c.Key]
	if !ok {
		return false, fmt.Errorf("key %q not found in context", c.Key)
	}
	f, err := toFloat64(val)
	if err != nil {
		return false, fmt.Errorf("key %q: %w", c.Key, err)
	}
	return f > c.Threshold, nil
}

// LessThan checks if the value at key (as a float64) is < threshold.
type LessThan struct {
	Key       string
	Threshold float64
}

func (c LessThan) Evaluate(ctx map[string]interface{}) (bool, error) {
	val, ok := ctx[c.Key]
	if !ok {
		return false, fmt.Errorf("key %q not found in context", c.Key)
	}
	f, err := toFloat64(val)
	if err != nil {
		return false, fmt.Errorf("key %q: %w", c.Key, err)
	}
	return f < c.Threshold, nil
}

// In checks if the value at key is in the provided slice.
type In struct {
	Key   string
	Slice []interface{}
}

func (c In) Evaluate(ctx map[string]interface{}) (bool, error) {
	val, ok := ctx[c.Key]
	if !ok {
		return false, fmt.Errorf("key %q not found in context", c.Key)
	}
	for _, item := range c.Slice {
		if reflect.DeepEqual(val, item) {
			return true, nil
		}
	}
	return false, nil
}

// Matches checks if the string value matches a regex pattern.
// (Optional: could be included if needed; we'll omit for brevity.)

// ----------------------------------------------------------------------
// Logical combinators
// ----------------------------------------------------------------------

// And combines multiple conditions with logical AND.
type And []Condition

func (a And) Evaluate(ctx map[string]interface{}) (bool, error) {
	for _, cond := range a {
		ok, err := cond.Evaluate(ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// Or combines multiple conditions with logical OR.
type Or []Condition

func (o Or) Evaluate(ctx map[string]interface{}) (bool, error) {
	for _, cond := range o {
		ok, err := cond.Evaluate(ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// Not negates a condition.
type Not struct {
	Condition Condition
}

func (n Not) Evaluate(ctx map[string]interface{}) (bool, error) {
	ok, err := n.Condition.Evaluate(ctx)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

// ----------------------------------------------------------------------
// Builder API
// ----------------------------------------------------------------------

// Builder provides a fluent interface for constructing conditions.
type Builder struct {
	cond Condition
	err  error
}

// NewBuilder starts a new condition builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Eq adds an equality condition.
func (b *Builder) Eq(key string, expected interface{}) *Builder {
	b.cond = b.append(b.cond, Equals{Key: key, Expected: expected})
	return b
}

// Neq adds a not‑equal condition.
func (b *Builder) Neq(key string, expected interface{}) *Builder {
	b.cond = b.append(b.cond, NotEquals{Key: key, Expected: expected})
	return b
}

// Gt adds a greater‑than condition.
func (b *Builder) Gt(key string, threshold float64) *Builder {
	b.cond = b.append(b.cond, GreaterThan{Key: key, Threshold: threshold})
	return b
}

// Lt adds a less‑than condition.
func (b *Builder) Lt(key string, threshold float64) *Builder {
	b.cond = b.append(b.cond, LessThan{Key: key, Threshold: threshold})
	return b
}

// In adds an in‑slice condition.
func (b *Builder) In(key string, slice ...interface{}) *Builder {
	b.cond = b.append(b.cond, In{Key: key, Slice: slice})
	return b
}

// And combines all previously added conditions into an And.
func (b *Builder) And() *Builder {
	if and, ok := b.cond.(And); ok {
		return b
	}
	if b.cond != nil {
		b.cond = And{b.cond}
	}
	return b
}

// Or combines all previously added conditions into an Or.
func (b *Builder) Or() *Builder {
	if or, ok := b.cond.(Or); ok {
		return b
	}
	if b.cond != nil {
		b.cond = Or{b.cond}
	}
	return b
}

// Not wraps the current condition with a Not.
func (b *Builder) Not() *Builder {
	if b.cond != nil {
		b.cond = Not{Condition: b.cond}
	}
	return b
}

// Build returns the final Condition and any accumulated error.
func (b *Builder) Build() (Condition, error) {
	return b.cond, b.err
}

// MustBuild returns the condition and panics if there was an error.
func (b *Builder) MustBuild() Condition {
	if b.err != nil {
		panic(b.err)
	}
	return b.cond
}

// append combines existing condition with a new leaf.
// If existing is nil, returns the leaf. If existing is an And/Or, appends.
// Otherwise, creates an And with existing and leaf.
func (b *Builder) append(existing Condition, leaf Condition) Condition {
	if existing == nil {
		return leaf
	}
	switch e := existing.(type) {
	case And:
		return append(e, leaf)
	case Or:
		// For simplicity, we treat Or as a single condition and create an And
		return And{existing, leaf}
	default:
		return And{existing, leaf}
	}
}

// ----------------------------------------------------------------------
// Helper functions
// ----------------------------------------------------------------------

// toFloat64 attempts to convert v to a float64.
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int8:
		return float64(val), nil
	case int16:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint8:
		return float64(val), nil
	case uint16:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert %v to float64", v)
	}
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     ctx := map[string]interface{}{
//         "age":   25,
//         "name":  "Alice",
//         "tags":  []string{"go", "dev"},
//     }
//
//     cond := condition.And{
//         condition.GreaterThan{Key: "age", Threshold: 18},
//         condition.Equals{Key: "name", Expected: "Alice"},
//     }
//     ok, _ := cond.Evaluate(ctx)
//     fmt.Println(ok) // true
//
//     // Using builder
//     builder := condition.NewBuilder().
//         Gt("age", 18).
//         Eq("name", "Alice").
//         And()
//     cond2 := builder.MustBuild()
//     ok2, _ := cond2.Evaluate(ctx)
//     fmt.Println(ok2) // true
// }