package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CompositeIntError combines multiple integer-related errors with improved formatting
type CompositeIntError struct {
	Errors []error
	Prefix string
	Values []int // Associated integer values that caused errors
}

// NewCompositeIntError creates a new CompositeIntError
func NewCompositeIntError(prefix string) *CompositeIntError {
	return &CompositeIntError{
		Errors: make([]error, 0),
		Prefix: prefix,
		Values: make([]int, 0),
	}
}

func (ce *CompositeIntError) Error() string {
	if len(ce.Errors) == 0 {
		return "no errors"
	}

	var builder strings.Builder
	if ce.Prefix != "" {
		builder.WriteString(ce.Prefix)
		builder.WriteString(": ")
	}

	for i, err := range ce.Errors {
		if i > 0 {
			builder.WriteString("; ")
		}
		if i < len(ce.Values) {
			builder.WriteString(fmt.Sprintf("[value=%d] %v", ce.Values[i], err))
		} else {
			builder.WriteString(fmt.Sprintf("[%d] %v", i+1, err))
		}
	}

	return builder.String()
}

// Add adds an error with associated integer value
func (ce *CompositeIntError) Add(err error, value int) {
	if err != nil {
		ce.Errors = append(ce.Errors, err)
		ce.Values = append(ce.Values, value)
	}
}

// AddError adds an error without associated value
func (ce *CompositeIntError) AddError(err error) {
	if err != nil {
		ce.Errors = append(ce.Errors, err)
	}
}

// HasErrors returns true if there are any errors
func (ce *CompositeIntError) HasErrors() bool {
	return len(ce.Errors) > 0
}

// Unwrap returns the underlying errors
func (ce *CompositeIntError) Unwrap() []error {
	return ce.Errors
}

// As implements errors.As
func (ce *CompositeIntError) As(target any) bool {
	for _, err := range ce.Errors {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

// Is implements errors.Is
func (ce *CompositeIntError) Is(target error) bool {
	for _, err := range ce.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of errors
func (ce *CompositeIntError) ErrorCount() int {
	return len(ce.Errors)
}

// IntCollection manages a collection of integers with statistical operations
type IntCollection struct {
	mu     sync.RWMutex
	values []int
	sorted bool
}

// NewIntCollection creates a new integer collection
func NewIntCollection(values ...int) *IntCollection {
	return &IntCollection{
		values: values,
		sorted: false,
	}
}

// Add adds values to the collection
func (ic *IntCollection) Add(values ...int) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.values = append(ic.values, values...)
	ic.sorted = false
}

// Len returns the number of values
func (ic *IntCollection) Len() int {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return len(ic.values)
}

// Values returns a copy of the values
func (ic *IntCollection) Values() []int {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	values := make([]int, len(ic.values))
	copy(values, ic.values)
	return values
}

// Sum calculates the sum of all values
func (ic *IntCollection) Sum() int {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	sum := 0
	for _, v := range ic.values {
		sum += v
	}
	return sum
}

// Average calculates the average of all values
func (ic *IntCollection) Average() float64 {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) == 0 {
		return 0
	}
	return float64(ic.Sum()) / float64(len(ic.values))
}

// Median calculates the median value
func (ic *IntCollection) Median() float64 {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) == 0 {
		return 0
	}

	ic.sortLocked()

	if len(ic.values)%2 == 1 {
		return float64(ic.values[len(ic.values)/2])
	}

	middle := len(ic.values) / 2
	return float64(ic.values[middle-1]+ic.values[middle]) / 2.0
}

// Mode calculates the mode (most frequent value)
func (ic *IntCollection) Mode() []int {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) == 0 {
		return nil
	}

	freq := make(map[int]int)
	maxFreq := 0

	for _, v := range ic.values {
		freq[v]++
		if freq[v] > maxFreq {
			maxFreq = freq[v]
		}
	}

	var modes []int
	for v, f := range freq {
		if f == maxFreq {
			modes = append(modes, v)
		}
	}

	return modes
}

// Min returns the minimum value
func (ic *IntCollection) Min() (int, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) == 0 {
		return 0, false
	}

	ic.sortLocked()
	return ic.values[0], true
}

// Max returns the maximum value
func (ic *IntCollection) Max() (int, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) == 0 {
		return 0, false
	}

	ic.sortLocked()
	return ic.values[len(ic.values)-1], true
}

// Range returns the range (max - min)
func (ic *IntCollection) Range() (int, bool) {
	min, ok1 := ic.Min()
	max, ok2 := ic.Max()
	if !ok1 || !ok2 {
		return 0, false
	}
	return max - min, true
}

// StandardDeviation calculates the population standard deviation
func (ic *IntCollection) StandardDeviation() float64 {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) < 2 {
		return 0
	}

	mean := ic.Average()
	var sumSquares float64
	for _, v := range ic.values {
		diff := float64(v) - mean
		sumSquares += diff * diff
	}

	return math.Sqrt(sumSquares / float64(len(ic.values)))
}

// Percentile calculates the value at the given percentile (0-100)
func (ic *IntCollection) Percentile(p float64) (float64, error) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	if len(ic.values) == 0 {
		return 0, errors.New("no values in collection")
	}
	if p < 0 || p > 100 {
		return 0, fmt.Errorf("percentile must be between 0 and 100, got %f", p)
	}

	ic.sortLocked()

	if p == 0 {
		return float64(ic.values[0]), nil
	}
	if p == 100 {
		return float64(ic.values[len(ic.values)-1]), nil
	}

	index := (p / 100) * float64(len(ic.values)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return float64(ic.values[lower]), nil
	}

	// Linear interpolation
	lowerValue := float64(ic.values[lower])
	upperValue := float64(ic.values[upper])
	weight := index - float64(lower)

	return lowerValue + (upperValue-lowerValue)*weight, nil
}

// Filter returns a new collection with values that match the predicate
func (ic *IntCollection) Filter(predicate func(int) bool) *IntCollection {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	var filtered []int
	for _, v := range ic.values {
		if predicate(v) {
			filtered = append(filtered, v)
		}
	}

	return NewIntCollection(filtered...)
}

// Map applies a function to each value and returns a new collection
func (ic *IntCollection) Map(mapper func(int) int) *IntCollection {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	mapped := make([]int, len(ic.values))
	for i, v := range ic.values {
		mapped[i] = mapper(v)
	}

	return NewIntCollection(mapped...)
}

// JSON returns the collection as a JSON array
func (ic *IntCollection) JSON() ([]byte, error) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return json.Marshal(ic.values)
}

func (ic *IntCollection) sortLocked() {
	if !ic.sorted {
		sort.Ints(ic.values)
		ic.sorted = true
	}
}

// RandomIntGenerator provides thread-safe random integer generation
type RandomIntGenerator struct {
	mu        sync.Mutex
	rand      *rand.Rand
	seed      int64
	callCount atomic.Int64
	config    RandomIntConfig
}

// RandomIntConfig holds configuration for random integer generation
type RandomIntConfig struct {
	Seed      int64         // Random seed (0 for time-based)
	Min       int           // Minimum value (inclusive)
	Max       int           // Maximum value (inclusive)
	AllowZero bool          // Allow zero values
	AllowNeg  bool          // Allow negative values
	RetryMax  int           // Maximum retries for constrained generation
	Timeout   time.Duration // Timeout for context-aware generation
}

// DefaultRandomConfig returns a safe default configuration
func DefaultRandomConfig() RandomIntConfig {
	return RandomIntConfig{
		Seed:      time.Now().UnixNano(),
		Min:       0,
		Max:       100,
		AllowZero: true,
		AllowNeg:  false,
		RetryMax:  1000,
		Timeout:   5 * time.Second,
	}
}

// NewRandomIntGenerator creates a new random integer generator
func NewRandomIntGenerator(config RandomIntConfig) *RandomIntGenerator {
	if config.Seed == 0 {
		config.Seed = time.Now().UnixNano()
	}

	return &RandomIntGenerator{
		rand:   rand.New(rand.NewSource(config.Seed)),
		seed:   config.Seed,
		config: config,
	}
}

// Generate generates a random integer within configured bounds
func (rg *RandomIntGenerator) Generate() (int, error) {
	return rg.GenerateWithBounds(rg.config.Min, rg.config.Max)
}

// GenerateWithBounds generates a random integer within custom bounds
func (rg *RandomIntGenerator) GenerateWithBounds(min, max int) (int, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	rg.callCount.Add(1)

	if min > max {
		min, max = max, min
	}

	value := rg.rand.Intn(max-min+1) + min

	// Apply constraints
	if !rg.config.AllowZero && value == 0 {
		return rg.regenerateWithConstraint(min, max, func(v int) bool {
			return v != 0
		})
	}

	if !rg.config.AllowNeg && value < 0 {
		return rg.regenerateWithConstraint(min, max, func(v int) bool {
			return v >= 0
		})
	}

	return value, nil
}

// GenerateWithContext generates a random integer with context cancellation
func (rg *RandomIntGenerator) GenerateWithContext(ctx context.Context) (int, error) {
	return rg.GenerateWithContextAndBounds(ctx, rg.config.Min, rg.config.Max)
}

// GenerateWithContextAndBounds generates with context and custom bounds
func (rg *RandomIntGenerator) GenerateWithContextAndBounds(ctx context.Context, min, max int) (int, error) {
	timeout := rg.config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for attempt := 0; attempt < rg.config.RetryMax; attempt++ {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("random generation timeout: %w", ctx.Err())
		default:
			value, err := rg.GenerateWithBounds(min, max)
			if err == nil {
				return value, nil
			}

			// Exponential backoff with jitter
			if attempt < rg.config.RetryMax-1 {
				backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Millisecond
				jitter := time.Duration(rg.rand.Int63n(int64(backoff / 2)))
				time.Sleep(backoff + jitter)
			}
		}
	}

	return 0, fmt.Errorf("failed to generate random integer after %d attempts", rg.config.RetryMax)
}

// GenerateMany generates multiple random integers
func (rg *RandomIntGenerator) GenerateMany(count int) ([]int, error) {
	return rg.GenerateManyWithBounds(count, rg.config.Min, rg.config.Max)
}

// GenerateManyWithBounds generates multiple integers with custom bounds
func (rg *RandomIntGenerator) GenerateManyWithBounds(count, min, max int) ([]int, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	if count <= 0 {
		return nil, fmt.Errorf("count must be positive, got %d", count)
	}

	results := make([]int, count)
	for i := 0; i < count; i++ {
		value := rg.rand.Intn(max-min+1) + min

		// Apply constraints with retry
		if (!rg.config.AllowZero && value == 0) || (!rg.config.AllowNeg && value < 0) {
			for retry := 0; retry < rg.config.RetryMax; retry++ {
				value = rg.rand.Intn(max-min+1) + min
				if (rg.config.AllowZero || value != 0) && (rg.config.AllowNeg || value >= 0) {
					break
				}
			}
		}

		results[i] = value
		rg.callCount.Add(1)
	}

	return results, nil
}

// GenerateUnique generates unique random integers
func (rg *RandomIntGenerator) GenerateUnique(count int) ([]int, error) {
	return rg.GenerateUniqueWithBounds(count, rg.config.Min, rg.config.Max)
}

// GenerateUniqueWithBounds generates unique integers with custom bounds
func (rg *RandomIntGenerator) GenerateUniqueWithBounds(count, min, max int) ([]int, error) {
	if count > (max - min + 1) {
		return nil, fmt.Errorf("cannot generate %d unique values in range [%d, %d]", count, min, max)
	}

	rg.mu.Lock()
	defer rg.mu.Unlock()

	generated := make(map[int]bool)
	results := make([]int, 0, count)

	for len(results) < count {
		value := rg.rand.Intn(max-min+1) + min

		if !generated[value] {
			// Apply constraints
			if (!rg.config.AllowZero && value == 0) || (!rg.config.AllowNeg && value < 0) {
				continue
			}

			generated[value] = true
			results = append(results, value)
			rg.callCount.Add(1)
		}

		// Safety check
		if len(generated) > (max-min+1)*2 {
			return nil, fmt.Errorf("failed to generate %d unique values after excessive attempts", count)
		}
	}

	return results, nil
}

// Seed returns the current seed
func (rg *RandomIntGenerator) Seed() int64 {
	return rg.seed
}

// CallCount returns the number of random numbers generated
func (rg *RandomIntGenerator) CallCount() int64 {
	return rg.callCount.Load()
}

// Reset resets the generator with optional new seed
func (rg *RandomIntGenerator) Reset(seed int64) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	rg.seed = seed
	rg.rand = rand.New(rand.NewSource(seed))
	rg.callCount.Store(0)
}

func (rg *RandomIntGenerator) regenerateWithConstraint(min, max int, constraint func(int) bool) (int, error) {
	for retry := 0; retry < rg.config.RetryMax; retry++ {
		value := rg.rand.Intn(max-min+1) + min
		if constraint(value) {
			return value, nil
		}
	}

	return 0, fmt.Errorf("failed to generate value satisfying constraints after %d attempts", rg.config.RetryMax)
}

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

// IntStats provides statistical analysis for integer collections
type IntStats struct {
	Count    int     `json:"count"`
	Sum      int     `json:"sum"`
	Mean     float64 `json:"mean"`
	Median   float64 `json:"median"`
	Mode     []int   `json:"mode"`
	Min      int     `json:"min"`
	Max      int     `json:"max"`
	Range    int     `json:"range"`
	StdDev   float64 `json:"std_dev"`
	Variance float64 `json:"variance"`
	Q1       float64 `json:"q1"`  // First quartile
	Q3       float64 `json:"q3"`  // Third quartile
	IQR      float64 `json:"iqr"` // Interquartile range
}

// Analyze analyzes a collection of integers
func (iu *IntUtilities) Analyze(values []int) *IntStats {
	if len(values) == 0 {
		return &IntStats{}
	}

	stats := &IntStats{
		Count: len(values),
		Mode:  make([]int, 0),
	}

	// Create a sorted copy for calculations
	sorted := make([]int, len(values))
	copy(sorted, values)
	sort.Ints(sorted)

	// Basic calculations
	stats.Min = sorted[0]
	stats.Max = sorted[len(sorted)-1]
	stats.Range = stats.Max - stats.Min

	for _, v := range values {
		stats.Sum += v
	}
	stats.Mean = float64(stats.Sum) / float64(stats.Count)

	// Median
	if stats.Count%2 == 1 {
		stats.Median = float64(sorted[stats.Count/2])
	} else {
		stats.Median = float64(sorted[stats.Count/2-1]+sorted[stats.Count/2]) / 2.0
	}

	// Mode
	freq := make(map[int]int)
	maxFreq := 0
	for _, v := range values {
		freq[v]++
		if freq[v] > maxFreq {
			maxFreq = freq[v]
		}
	}
	for v, f := range freq {
		if f == maxFreq {
			stats.Mode = append(stats.Mode, v)
		}
	}
	sort.Ints(stats.Mode)

	// Variance and standard deviation
	var sumSquares float64
	for _, v := range values {
		diff := float64(v) - stats.Mean
		sumSquares += diff * diff
	}
	stats.Variance = sumSquares / float64(stats.Count)
	stats.StdDev = math.Sqrt(stats.Variance)

	// Quartiles
	if stats.Count >= 4 {
		mid := stats.Count / 2
		lowerHalf := sorted[:mid]
		upperHalf := sorted[mid+stats.Count%2:]

		stats.Q1 = median(lowerHalf)
		stats.Q3 = median(upperHalf)
		stats.IQR = stats.Q3 - stats.Q1
	}

	return stats
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

func median(values []int) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return float64(values[n/2])
	}
	return float64(values[n/2-1]+values[n/2]) / 2.0
}

// Example usage function
func ExampleIntUtilities() {
	// Composite error example
	compositeErr := NewCompositeIntError("integer processing errors")
	compositeErr.Add(fmt.Errorf("value out of range"), 150)
	compositeErr.Add(fmt.Errorf("invalid format"), -1)

	if compositeErr.HasErrors() {
		fmt.Printf("Errors: %v\n", compositeErr)
	}

	// Integer collection example
	collection := NewIntCollection(1, 2, 3, 4, 5, 5, 7, 8, 9, 10)
	collection.Add(6, 11, 12)

	fmt.Printf("Sum: %d\n", collection.Sum())
	fmt.Printf("Average: %.2f\n", collection.Average())
	fmt.Printf("Median: %.2f\n", collection.Median())

	// Random integer generation
	config := DefaultRandomConfig()
	config.Min = 1
	config.Max = 100
	generator := NewRandomIntGenerator(config)

	// Generate single random integer
	if randInt, err := generator.Generate(); err == nil {
		fmt.Printf("Random integer: %d\n", randInt)
	}

	// Generate multiple unique integers
	if randInts, err := generator.GenerateUnique(10); err == nil {
		fmt.Printf("Unique random integers: %v\n", randInts)
	}

	// Statistical analysis
	utils := NewIntUtilities()
	stats := utils.Analyze([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	if jsonBytes, err := json.MarshalIndent(stats, "", "  "); err == nil {
		fmt.Printf("Statistics: %s\n", string(jsonBytes))
	}

	// Validation example
	validator := NewIntValidator()
	if valid, err := validator.Validate(42, "positive", "even"); valid {
		fmt.Println("Validation passed!")
	} else {
		fmt.Printf("Validation failed: %v\n", err)
	}
}
