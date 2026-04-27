package generator

import (
	"sync"
	"testing"
	"time"
)

// --- Вспомогательные функции ---

func newDefaultGen(t *testing.T) *Generator {
	t.Helper()
	g, err := New(Config{DatacenterID: 1, MachineID: 1})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return g
}

// --- Тесты создания генератора ---

func TestNew_ValidConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"zero IDs", Config{DatacenterID: 0, MachineID: 0}},
		{"max IDs", Config{DatacenterID: MaxDatacenterID, MachineID: MaxMachineID}},
		{"mid IDs", Config{DatacenterID: 15, MachineID: 15}},
		{"custom epoch", Config{DatacenterID: 1, MachineID: 1, Epoch: 1_600_000_000_000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := New(tc.cfg)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if g == nil {
				t.Error("expected non-nil generator")
			}
		})
	}
}

func TestNew_InvalidDatacenterID(t *testing.T) {
	cases := []int64{-1, MaxDatacenterID + 1, 100}
	for _, id := range cases {
		_, err := New(Config{DatacenterID: id, MachineID: 0})
		if err == nil {
			t.Errorf("expected error for DatacenterID=%d, got nil", id)
		}
	}
}

func TestNew_InvalidMachineID(t *testing.T) {
	cases := []int64{-1, MaxMachineID + 1, 100}
	for _, id := range cases {
		_, err := New(Config{DatacenterID: 0, MachineID: id})
		if err == nil {
			t.Errorf("expected error for MachineID=%d, got nil", id)
		}
	}
}

func TestNew_DefaultEpoch(t *testing.T) {
	g, _ := New(Config{})
	if g.epoch != TwitterEpoch {
		t.Errorf("expected epoch %d, got %d", TwitterEpoch, g.epoch)
	}
}

func TestNew_CustomEpoch(t *testing.T) {
	custom := int64(1_700_000_000_000)
	g, _ := New(Config{Epoch: custom})
	if g.epoch != custom {
		t.Errorf("expected epoch %d, got %d", custom, g.epoch)
	}
}

// --- Тесты генерации ID ---

func TestNextID_NotZero(t *testing.T) {
	g := newDefaultGen(t)
	id, err := g.NextID()
	if err != nil {
		t.Fatalf("NextID() error: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestNextID_PositiveSign(t *testing.T) {
	g := newDefaultGen(t)
	for i := 0; i < 100; i++ {
		id, err := g.NextID()
		if err != nil {
			t.Fatalf("NextID() error: %v", err)
		}
		if id < 0 {
			t.Errorf("ID must be positive (sign bit=0), got %d", id)
		}
	}
}

func TestNextID_Monotonic(t *testing.T) {
	g := newDefaultGen(t)
	prev, _ := g.NextID()
	for i := 0; i < 10_000; i++ {
		id, err := g.NextID()
		if err != nil {
			t.Fatalf("NextID() error: %v", err)
		}
		if id <= prev {
			t.Errorf("IDs are not monotonically increasing: prev=%d, cur=%d", prev, id)
		}
		prev = id
	}
}

func TestNextID_Unique(t *testing.T) {
	g := newDefaultGen(t)
	const n = 10_000
	seen := make(map[int64]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := g.NextID()
		if err != nil {
			t.Fatalf("NextID() error: %v", err)
		}
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate ID detected: %d", id)
		}
		seen[id] = struct{}{}
	}
}

// --- Тесты разложения ID ---

func TestDecompose_DatacenterAndMachine(t *testing.T) {
	g, _ := New(Config{DatacenterID: 7, MachineID: 13})
	id, _ := g.NextID()
	d := g.Decompose(id)

	if d.DatacenterID != 7 {
		t.Errorf("expected DatacenterID=7, got %d", d.DatacenterID)
	}
	if d.MachineID != 13 {
		t.Errorf("expected MachineID=13, got %d", d.MachineID)
	}
}

func TestDecompose_TimestampReasonable(t *testing.T) {
	g := newDefaultGen(t)
	before := time.Now().UnixMilli()
	id, _ := g.NextID()
	after := time.Now().UnixMilli()

	d := g.Decompose(id)
	if d.Timestamp < before || d.Timestamp > after {
		t.Errorf("timestamp %d out of range [%d, %d]", d.Timestamp, before, after)
	}
}

func TestDecompose_SequenceRange(t *testing.T) {
	g := newDefaultGen(t)
	for i := 0; i < 1000; i++ {
		id, _ := g.NextID()
		d := g.Decompose(id)
		if d.Sequence < 0 || d.Sequence > MaxSequence {
			t.Errorf("sequence %d out of range [0, %d]", d.Sequence, MaxSequence)
		}
	}
}

func TestDecompose_IDField(t *testing.T) {
	g := newDefaultGen(t)
	id, _ := g.NextID()
	d := g.Decompose(id)
	if d.ID != id {
		t.Errorf("Decompose ID mismatch: got %d, want %d", d.ID, id)
	}
}

// --- Тесты кастомной эпохи ---

func TestCustomEpoch_TimestampOffset(t *testing.T) {
	// Устанавливаем эпоху в прошлое — временная метка должна быть больше
	epoch1 := int64(1_000_000_000_000) // более ранняя эпоха
	epoch2 := int64(1_600_000_000_000) // более поздняя эпоха

	g1, _ := New(Config{DatacenterID: 1, MachineID: 1, Epoch: epoch1})
	g2, _ := New(Config{DatacenterID: 1, MachineID: 1, Epoch: epoch2})

	id1, _ := g1.NextID()
	id2, _ := g2.NextID()

	// g1 имеет более раннюю эпоху → бо́льшая метка → бо́льший ID
	if id1 <= id2 {
		t.Errorf("expected id1 (%d) > id2 (%d) due to earlier epoch", id1, id2)
	}
}

// --- Конкурентный тест ---

func TestNextID_Concurrent_Unique(t *testing.T) {
	g := newDefaultGen(t)
	const goroutines = 20
	const perGoroutine = 500

	var mu sync.Mutex
	seen := make(map[int64]struct{}, goroutines*perGoroutine)
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]int64, 0, perGoroutine)
			for j := 0; j < perGoroutine; j++ {
				id, err := g.NextID()
				if err != nil {
					errCh <- err
					return
				}
				local = append(local, id)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range local {
				if _, dup := seen[id]; dup {
					t.Errorf("duplicate ID in concurrent test: %d", id)
				}
				seen[id] = struct{}{}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("goroutine error: %v", err)
	}

	expected := goroutines * perGoroutine
	if len(seen) != expected {
		t.Errorf("expected %d unique IDs, got %d", expected, len(seen))
	}
}

// --- Тест максимальных значений констант ---

func TestConstants(t *testing.T) {
	if MaxDatacenterID != 31 {
		t.Errorf("MaxDatacenterID expected 31, got %d", MaxDatacenterID)
	}
	if MaxMachineID != 31 {
		t.Errorf("MaxMachineID expected 31, got %d", MaxMachineID)
	}
	if MaxSequence != 4095 {
		t.Errorf("MaxSequence expected 4095, got %d", MaxSequence)
	}
}

// --- Benchmark ---

func BenchmarkNextID(b *testing.B) {
	g, _ := New(Config{DatacenterID: 1, MachineID: 1})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.NextID()
	}
}

func BenchmarkNextID_Parallel(b *testing.B) {
	g, _ := New(Config{DatacenterID: 1, MachineID: 1})
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = g.NextID()
		}
	})
}
