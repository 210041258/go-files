package testutils

import (
    "fmt"
    "math"
    "sort"
    "strconv"
    "strings"
    "sync"
)

// IntUtilities provides various integer utility functions
type IntUtilities struct{}

// NewIntUtilities creates a new integer utilities instance
func NewIntUtilities() *IntUtilities {
    return &IntUtilities{}
}

// ParseInts parses a comma-separated string of integers
func (iu *IntUtilities) ParseInts(s string) ([]int, *CompositeIntError) {
    if s == "" {
        return nil, nil
    }

    parts := strings.Split(s, ",")
    results := make([]int, 0, len(parts))
    errors := NewCompositeIntError("parse errors")

    for i, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }

        // Check for range syntax (e.g., "1-5")
        if strings.Contains(part, "-") {
            rangeParts := strings.Split(part, "-")
            if len(rangeParts) == 2 {
                start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
                end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))

                if err1 != nil || err2 != nil {
                    errors.Add(fmt.Errorf("invalid range format: %s", part), 0)
                    continue
                }

                if start > end {
                    start, end = end, start
                }

                for v := start; v <= end; v++ {
                    results = append(results, v)
                }
                continue
            }
        }

        value, err := strconv.Atoi(part)
        if err != nil {
            errors.Add(fmt.Errorf("invalid integer at position %d: %s", i, part), 0)
            continue
        }

        results = append(results, value)
    }

    if errors.HasErrors() {
        return results, errors
    }

    return results, nil
}

// IsPrime checks if a number is prime
func (iu *IntUtilities) IsPrime(n int) bool {
    if n <= 1 {
        return false
    }
    if n <= 3 {
        return true
    }
    if n%2 == 0 || n%3 == 0 {
        return false
    }

    limit := int(math.Sqrt(float64(n)))
    for i := 5; i <= limit; i += 6 {
        if n%i == 0 || n%(i+2) == 0 {
            return false
        }
    }

    return true
}

// GCD calculates the greatest common divisor
func (iu *IntUtilities) GCD(a, b int) int {
    for b != 0 {
        a, b = b, a%b
    }
    return a
}

// LCM calculates the least common multiple
func (iu *IntUtilities) LCM(a, b int) int {
    if a == 0 || b == 0 {
        return 0
    }
    return abs(a*b) / iu.GCD(a, b)
}

// Factors returns all factors of a number
func (iu *IntUtilities) Factors(n int) []int {
    if n == 0 {
        return []int{}
    }

    n = abs(n)
    var factors []int

    limit := int(math.Sqrt(float64(n)))
    for i := 1; i <= limit; i++ {
        if n%i == 0 {
            factors = append(factors, i)
            if i != n/i {
                factors = append(factors, n/i)
            }
        }
    }

    sort.Ints(factors)
    return factors
}

// Fibonacci generates Fibonacci numbers up to limit or count
func (iu *IntUtilities) Fibonacci(limit int, maxCount int) []int {
    if limit <= 0 && maxCount <= 0 {
        return []int{}
    }

    var result []int
    a, b := 0, 1

    for count := 0; (limit <= 0 || b <= limit) && (maxCount <= 0 || count < maxCount); count++ {
        result = append(result, b)
        a, b = b, a+b
    }

    return result
}

// IntValidator provides validation for integer values
type IntValidator struct {
    mu    sync.RWMutex
    rules []ValidationRule
}

// ValidationRule defines a rule for validating integers
type ValidationRule struct {
    Name        string
    Description string
    Validator   func(int) (bool, string)
}

// NewIntValidator creates a new integer validator
func NewIntValidator() *IntValidator {
    return &IntValidator{
        rules: []ValidationRule{
            {
                Name:        "not_zero",
                Description: "Value must not be zero",
                Validator: func(v int) (bool, string) {
                    return v != 0, "value cannot be zero"
                },
            },
            {
                Name:        "positive",
                Description: "Value must be positive",
                Validator: func(v int) (bool, string) {
                    return v > 0, "value must be positive"
                },
            },
            {
                Name:        "even",
                Description: "Value must be even",
                Validator: func(v int) (bool, string) {
                    return v%2 == 0, "value must be even"
                },
            },
            {
                Name:        "odd",
                Description: "Value must be odd",
                Validator: func(v int) (bool, string) {
                    return v%2 == 1, "value must be odd"
                },
            },
        },
    }
}

// Validate validates a value against named rules
func (iv *IntValidator) Validate(value int, ruleNames ...string) (bool, *CompositeIntError) {
    iv.mu.RLock()
    defer iv.mu.RUnlock()

    errors := NewCompositeIntError("validation failed")
    allValid := true

    if len(ruleNames) == 0 {
        // Apply all rules
        for _, rule := range iv.rules {
            if valid, msg := rule.Validator(value); !valid {
                errors.Add(fmt.Errorf("%s: %s", rule.Name, msg), value)
                allValid = false
            }
        }
    } else {
        // Apply specific rules
        ruleMap := make(map[string]ValidationRule)
        for _, rule := range iv.rules {
            ruleMap[rule.Name] = rule
        }

        for _, name := range ruleNames {
            if rule, exists := ruleMap[name]; exists {
                if valid, msg := rule.Validator(value); !valid {
                    errors.Add(fmt.Errorf("%s: %s", name, msg), value)
                    allValid = false
                }
            } else {
                errors.Add(fmt.Errorf("unknown rule: %s", name), value)
                allValid = false
            }
        }
    }

    if allValid {
        return true, nil
    }

    return false, errors
}

// AddRule adds a custom validation rule
func (iv *IntValidator) AddRule(name, description string, validator func(int) (bool, string)) {
    iv.mu.Lock()
    defer iv.mu.Unlock()

    iv.rules = append(iv.rules, ValidationRule{
        Name:        name,
        Description: description,
        Validator:   validator,
    })
}

// Helper functions

func abs(x int) int {
    if x < 0 {
        return -x
    }
    return x
}