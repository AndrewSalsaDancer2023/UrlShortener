package generator_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/generator"
)

func newGen(t *testing.T) *generator.Generator {
	t.Helper()
	g, err := generator.New(generator.Config{DatacenterID: 0, MachineID: 0})
	require.NoError(t, err)
	return g
}

// --- тесты конструктора ---

func TestNew_ValidConfig(t *testing.T) {
	cases := []generator.Config{
		{DatacenterID: 0, MachineID: 0},
		{DatacenterID: generator.MaxDatacenterID, MachineID: generator.MaxMachineID},
		{DatacenterID: 1, MachineID: 1},
	}
	for _, cfg := range cases {
		g, err := generator.New(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, g)
	}
}

func TestNew_InvalidDatacenterID(t *testing.T) {
	for _, id := range []int64{-1, generator.MaxDatacenterID + 1} {
		_, err := generator.New(generator.Config{DatacenterID: id})
		assert.Error(t, err, "DatacenterID=%d must be rejected", id)
	}
}

func TestNew_InvalidMachineID(t *testing.T) {
	for _, id := range []int64{-1, generator.MaxMachineID + 1} {
		_, err := generator.New(generator.Config{MachineID: id})
		assert.Error(t, err, "MachineID=%d must be rejected", id)
	}
}

// --- тесты NextID ---

func TestNextID_Positive(t *testing.T) {
	g := newGen(t)
	id, err := g.NextID()
	require.NoError(t, err)
	assert.Greater(t, id, int64(0), "ID must be positive (sign bit = 0)")
}

func TestNextID_Monotonic(t *testing.T) {
	g := newGen(t)
	prev, _ := g.NextID()
	for i := 0; i < 10_000; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		assert.Greater(t, id, prev, "IDs must be monotonically increasing")
		prev = id
	}
}

func TestNextID_Unique(t *testing.T) {
	g := newGen(t)
	const n = 10_000
	seen := make(map[int64]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		_, dup := seen[id]
		assert.False(t, dup, "duplicate ID: %d", id)
		seen[id] = struct{}{}
	}
}

func TestNextID_Concurrent_Unique(t *testing.T) {
	g := newGen(t)
	const goroutines, perGoroutine = 20, 500

	var mu sync.Mutex
	seen := make(map[int64]struct{}, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]int64, 0, perGoroutine)
			for j := 0; j < perGoroutine; j++ {
				id, err := g.NextID()
				require.NoError(t, err)
				local = append(local, id)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range local {
				_, dup := seen[id]
				assert.False(t, dup, "concurrent duplicate: %d", id)
				seen[id] = struct{}{}
			}
		}()
	}
	wg.Wait()
	assert.Len(t, seen, goroutines*perGoroutine)
}

func BenchmarkNextID(b *testing.B) {
	g, _ := generator.New(generator.Config{DatacenterID: 1, MachineID: 1})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.NextID()
	}
}
