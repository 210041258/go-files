package testutils


import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

// IntCollection manages a collection of integers with statistical operations
type IntCollection struct {
	mu     sync.RWMutex
	values []int
	sorted bool
}

// NewIntCollection creates a new integer collection
func NewIntCollection(values ...int) *IntCollection {
	// Create a copy to avoid external mutation
	cpy := make([]int, len(values))
	copy(cpy, values)
	return &IntCollection{
		values: cpy,
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

// ensureSorted safely sorts the collection if it is not already sorted.
// It handles upgrading the lock from read to write without race conditions.
func (ic *IntCollection) ensureSorted() {
	ic.mu.RLock()
	if ic.sorted {
		ic.mu.RUnlock()
		return
	}
	ic.mu.RUnlock()

	ic.mu.Lock()
	defer ic.mu.Unlock()
	// Double-check pattern
	if !ic.sorted {
		sort.Ints(ic.values)
		ic.sorted = true
	}
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
	ic.ensureSorted()

	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if len(ic.values) == 0 {
		return 0
	}

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
	ic.ensureSorted()

	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if len(ic.values) == 0 {
		return 0, false
	}
	return ic.values[0], true
}

// Max returns the maximum value
func (ic *IntCollection) Max() (int, bool) {
	ic.ensureSorted()

	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if len(ic.values) == 0 {
		return 0, false
	}
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

// Variance calculates the population variance
func (ic *IntCollection) Variance() float64 {
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

	return sumSquares / float64(len(ic.values))
}

// Percentile calculates the value at the given percentile (0-100)
func (ic *IntCollection) Percentile(p float64) (float64, error) {
	if p < 0 || p > 100 {
		return 0, fmt.Errorf("percentile must be between 0 and 100, got %f", p)
	}

	ic.ensureSorted()
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if len(ic.values) == 0 {
		return 0, fmt.New("no values in collection")
	}

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

// Analyze analyzes a collection of integers by leveraging IntCollection
func (iu *IntUtilities) Analyze(values []int) *IntStats {
	if len(values) == 0 {
		return &IntStats{}
	}

	ic := NewIntCollection(values...)
	stats := &IntStats{
		Count:    ic.Len(),
		Sum:      ic.Sum(),
		Mean:     ic.Average(),
		Median:   ic.Median(),
		Mode:     ic.Mode(),
		StdDev:   ic.StandardDeviation(),
		Variance: ic.Variance(),
	}

	if min, ok := ic.Min(); ok {
		stats.Min = min
	}
	if max, ok := ic.Max(); ok {
		stats.Max = max
	}
	if r, ok := ic.Range(); ok {
		stats.Range = r
	}

	// Calculate Quartiles manually since they need the sorted array
	sorted := ic.Values()
	sort.Ints(sorted)

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

// Helper functions

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
