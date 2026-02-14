package testutils

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
)

const (
	// Name is the name of round_robin balancer.
	Name = "custom_round_robin"
)

var logger = grpclog.Component("roundrobin")

func init() {
	balancer.Register(newBuilder())
}

type rrPickerBuilder struct{}

func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &rrPickerBuilder{}, base.Config{HealthCheck: true})
}

func (b *rrPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	// For unweighted round robin, we simply create a slice of subConns.
	var scs []balancer.SubConn
	for sc := range info.ReadySCs {
		scs = append(scs, sc)
	}

	return &rrPicker{
		subConns: scs,
		// Start at a random index? For simplicity, start at 0.
		next: 0,
	}
}

// rrPicker picks the next subConn in a round-robin manner.
type rrPicker struct {
	subConns []balancer.SubConn
	mu       sync.Mutex
	next     int
}

func (p *rrPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sc := p.subConns[p.next]
	p.next = (p.next + 1) % len(p.subConns)
	return balancer.PickResult{SubConn: sc}, nil
}

// WeightedItem represents a server with a weight for smooth weighted round-robin.
type WeightedItem struct {
	SubConn balancer.SubConn
	Weight  int
}

// SmoothWeightedRoundRobinPicker implements the smooth weighted round-robin algorithm.
// See: https://github.com/phusion/nginx/commit/27e94984486058d73157038f7950a0a36ecc6e35
type SmoothWeightedRoundRobinPicker struct {
	items          []*WeightedItem
	currentWeights []int
	mu             sync.Mutex
}

// NewSmoothWeightedRoundRobinPicker creates a picker with weighted round-robin.
func NewSmoothWeightedRoundRobinPicker(items []WeightedItem) *SmoothWeightedRoundRobinPicker {
	sw := &SmoothWeightedRoundRobinPicker{
		items:          make([]*WeightedItem, len(items)),
		currentWeights: make([]int, len(items)),
	}
	for i := range items {
		sw.items[i] = &items[i]
		sw.currentWeights[i] = 0
	}
	return sw
}

// Pick selects the next server using the smooth weighted algorithm.
func (p *SmoothWeightedRoundRobinPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	// Find the best item (highest current weight)
	bestIndex := -1
	total := 0
	for i, item := range p.items {
		p.currentWeights[i] += item.Weight
		total += item.Weight
		if bestIndex == -1 || p.currentWeights[i] > p.currentWeights[bestIndex] {
			bestIndex = i
		}
	}
	// Reduce the chosen one by total weight
	p.currentWeights[bestIndex] -= total

	return balancer.PickResult{SubConn: p.items[bestIndex].SubConn}, nil
}

// WeightedRRPickerBuilder builds a smooth weighted round-robin picker.
type WeightedRRPickerBuilder struct{}

func (b *WeightedRRPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	var items []WeightedItem
	for sc, scInfo := range info.ReadySCs {
		// Extract weight from address metadata.
		// The metadata can be set by the resolver via resolver.Address.Metadata.
		weight := 1 // default weight
		if scInfo.Address.Metadata != nil {
			if md, ok := scInfo.Address.Metadata.(map[string]interface{}); ok {
				if w, ok := md["weight"]; ok {
					if f, ok := w.(float64); ok {
						weight = int(f)
					} else if i, ok := w.(int); ok {
						weight = i
					}
				}
			}
		}
		items = append(items, WeightedItem{
			SubConn: sc,
			Weight:  weight,
		})
	}
	return NewSmoothWeightedRoundRobinPicker(items)
}

// RegisterWeightedRoundRobin registers the weighted round-robin balancer.
// To use it, set the balancer name to "weighted_round_robin" in the dial option.
func RegisterWeightedRoundRobin() {
	balancer.Register(base.NewBalancerBuilder("weighted_round_robin", &WeightedRRPickerBuilder{}, base.Config{HealthCheck: true}))
}

// Example usage (commented out):
// func main() {
//     // Register the weighted round-robin balancer.
//     roundrobin.RegisterWeightedRoundRobin()
//
//     // Dial with balancer name.
//     conn, err := grpc.Dial("etcd:///my-service",
//         grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"weighted_round_robin"}`),
//         grpc.WithInsecure(),
//     )
//     if err != nil {
//         log.Fatalf("did not connect: %v", err)
//     }
//     defer conn.Close()
//     // ...
// }