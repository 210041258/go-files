// Package reflectutil provides reflection utilities not found in the
// standard reflect package. It simplifies common tasks like inspecting
// struct fields, calling methods by name, and checking interface
// implementations.
package testutils

import (
	"errors"
	"reflect"
)

// ----------------------------------------------------------------------
// Basic helpers
// ----------------------------------------------------------------------

// TypeOf returns the reflect.Type of v, with special handling for nil.
// If v is nil, it returns nil.
func TypeOf(v any) reflect.Type {
	if v == nil {
		return nil
	}
	return reflect.TypeOf(v)
}

// ValueOf returns the reflect.Value of v, with special handling for nil.
// If v is nil, it returns a zero Value (not valid).
func ValueOf(v any) reflect.Value {
	if v == nil {
		return reflect.Value{}
	}
	return reflect.ValueOf(v)
}

// IsNil reports whether v is nil. It handles interface, pointer, map, slice,
// channel, and function values. For other types, it returns false.
func IsNil(v any) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return val.IsNil()
	}
	return false
}

// IsZero reports whether v is the zero value for its type.
func IsZero(v any) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	return val.IsZero()
}

// ----------------------------------------------------------------------
// Struct field access
// ----------------------------------------------------------------------

// GetField returns the value of the named field from the struct pointed to by v.
// v must be a pointer to a struct, or a struct. Returns an error if the field
// does not exist or cannot be accessed.
func GetField(v any, name string) (any, error) {
	val := reflect.ValueOf(v)
	// Dereference pointer if needed.
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, errors.New("v is not a struct or pointer to struct")
	}
	field := val.FieldByName(name)
	if !field.IsValid() {
		return nil, errors.New("field not found: " + name)
	}
	if !field.CanInterface() {
		return nil, errors.New("cannot access field: " + name)
	}
	return field.Interface(), nil
}

// SetField sets the value of the named field in the struct pointed to by v.
// v must be a pointer to a struct. The new value must be assignable to the field.
func SetField(v any, name string, value any) error {
	ptr := reflect.ValueOf(v)
	if ptr.Kind() != reflect.Ptr || ptr.IsNil() {
		return errors.New("v must be a non‑nil pointer to struct")
	}
	val := ptr.Elem()
	if val.Kind() != reflect.Struct {
		return errors.New("v must point to a struct")
	}
	field := val.FieldByName(name)
	if !field.IsValid() {
		return errors.New("field not found: " + name)
	}
	if !field.CanSet() {
		return errors.New("cannot set field: " + name)
	}
	newVal := reflect.ValueOf(value)
	if newVal.Type().AssignableTo(field.Type()) {
		field.Set(newVal)
	} else if newVal.Type().ConvertibleTo(field.Type()) {
		field.Set(newVal.Convert(field.Type()))
	} else {
		return errors.New("value type not assignable to field")
	}
	return nil
}

// StructFields returns a map of field names to their values for a struct.
// It only includes exported fields. If v is a pointer, it is dereferenced.
func StructFields(v any) (map[string]any, error) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, errors.New("v is not a struct or pointer to struct")
	}
	typ := val.Type()
	result := make(map[string]any)
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.CanInterface() {
			result[typ.Field(i).Name] = field.Interface()
		}
	}
	return result, nil
}

// ----------------------------------------------------------------------
// Method calling
// ----------------------------------------------------------------------

// CallMethod calls the named method on v with the given arguments.
// v can be any value; if it is a pointer, the method set includes pointer receivers.
// It returns the results as a slice of any. If the method does not exist,
// it returns an error.
func CallMethod(v any, name string, args ...any) ([]any, error) {
	val := reflect.ValueOf(v)
	method := val.MethodByName(name)
	if !method.IsValid() {
		return nil, errors.New("method not found: " + name)
	}
	// Prepare arguments.
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}
	// Call.
	out := method.Call(in)
	// Convert results to []any.
	result := make([]any, len(out))
	for i, r := range out {
		result[i] = r.Interface()
	}
	return result, nil
}

// Implements reports whether the value v implements the interface type represented
// by ifaceType. ifaceType must be a reflect.Type of an interface.
func Implements(v any, ifaceType reflect.Type) (bool, error) {
	if ifaceType.Kind() != reflect.Interface {
		return false, errors.New("ifaceType must be an interface type")
	}
	typ := reflect.TypeOf(v)
	if typ == nil {
		return false, nil // v is nil
	}
	return typ.Implements(ifaceType), nil
}

// ----------------------------------------------------------------------
// Indirect and type assertions
// ----------------------------------------------------------------------

// Indirect dereferences a pointer. If v is not a pointer, it returns v unchanged.
func Indirect(v any) any {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return v
	}
	return val.Elem().Interface()
}

// UnderlyingType returns the underlying type after removing all pointers.
// For example, given ***int, it returns reflect.TypeOf(int).
func UnderlyingType(v any) reflect.Type {
	typ := reflect.TypeOf(v)
	for typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     type User struct {
//         Name string
//         Age  int
//     }
//     u := &User{Name: "Alice", Age: 30}
//
//     name, _ := reflectutil.GetField(u, "Name")
//     fmt.Println(name) // "Alice"
//
//     reflectutil.SetField(u, "Age", 31)
//     fmt.Println(u.Age) // 31
//
//     fields, _ := reflectutil.StructFields(u)
//     fmt.Println(fields) // map[Age:31 Name:Alice]
//
//     // Call a method
//     type Greeter struct{}
//     func (g *Greeter) Hello(name string) string { return "Hello " + name }
//     g := &Greeter{}
//     results, _ := reflectutil.CallMethod(g, "Hello", "Bob")
//     fmt.Println(results[0]) // "Hello Bob"
// }