package idgenerator

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "urlshortener/internal/proto/idservice"
)

// fakeFetcher — фейковый BatchFetcher: не поднимает никакого gRPC,
// просто эмулирует сетевую задержку latency перед каждым ответом.
// Каждый батч содержит batchLen элементов с уникальным маркером номера
// вызова в старших разрядах — удобно для проверки, что батчи не
// повторяются и не перепутаны.
type fakeFetcher struct {
	batchLen int
	latency  time.Duration

	mu    sync.Mutex
	calls int
}

func (f *fakeFetcher) GetBatch(ctx context.Context, _ *pb.GetBatchRequest, _ ...grpc.CallOption) (*pb.GetBatchResponse, error) {
	select {
	case <-time.After(f.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()

	ids := make([]uint64, f.batchLen)
	for i := range ids {
		ids[i] = uint64(call)*1_000_000 + uint64(i)
	}
	return &pb.GetBatchResponse{Ids: ids}, nil
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// Базовый happy-path: последовательные id уникальны и идут по порядку
// внутри батча, батчи между собой не пересекаются.
func TestPool_NextID_ReturnsDistinctSequentialIDs(t *testing.T) {
	const batchLen = 5
	fetcher := &fakeFetcher{batchLen: batchLen, latency: time.Millisecond}
	pool := NewPool(fetcher, 2, time.Second)
	defer pool.Close()

	ctx := context.Background()
	seen := make(map[uint64]bool)

	for i := 0; i < batchLen*3; i++ { // забираем 3 полных батча
		id, err := pool.NextID(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen[id] {
			t.Fatalf("duplicate id returned: %d", id)
		}
		seen[id] = true
	}
}

// Главный тест: предвыборка должна скрывать сетевую задержку. Тратим
// первый батч полностью, даём фоновому Producer время предвыбрать
// следующий, и убеждаемся, что запрос СЛЕДУЮЩЕГО id после этого быстрый
// (не ждёт latency), а не медленный, как было бы без предвыборки.
func TestPool_PrefetchHidesNetworkLatency(t *testing.T) {
	const batchLen = 5
	const latency = 150 * time.Millisecond

	fetcher := &fakeFetcher{batchLen: batchLen, latency: latency}
	pool := NewPool(fetcher, 2, 2*time.Second)
	defer pool.Close()

	ctx := context.Background()

	// Тратим первый батч целиком — это должно занять примерно latency
	// (ожидание самого первого батча), после чего Producer уже начал
	// (и вскоре завершит) предвыборку второго батча в фоне.
	for i := 0; i < batchLen; i++ {
		if _, err := pool.NextID(ctx); err != nil {
			t.Fatalf("unexpected error draining first batch: %v", err)
		}
	}

	// Даём фоновому Producer время предвыбрать следующий батч.
	waitUntilAtLeast(t, fetcher, 2, 2*time.Second)

	// Теперь запрос следующего id должен быть быстрым — он берётся из
	// уже предвыбранного батча, а не ждёт сетевую задержку заново.
	start := time.Now()
	if _, err := pool.NextID(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > latency/2 {
		t.Fatalf("expected prefetch to hide network latency (elapsed << %v), got %v", latency, elapsed)
	}
}

// Без предвыборки (lookahead эффективно тот же механизм, но проверяем,
// что запрос ПОСЛЕ полного исчерпания без паузы на прогрев действительно
// ждёт сеть — это baseline, доказывающий, что предыдущий тест не проходит
// "просто так", а именно благодаря предвыборке.
func TestPool_WithoutPrefetchWarmup_BlocksOnNetwork(t *testing.T) {
	const batchLen = 3
	const latency = 150 * time.Millisecond

	fetcher := &fakeFetcher{batchLen: batchLen, latency: latency}
	pool := NewPool(fetcher, 1, 2*time.Second)
	defer pool.Close()

	ctx := context.Background()

	start := time.Now()
	// Самый первый вызов не может быть предвыбран заранее — Pool только
	// что создан, фоновый Producer ещё не успел ничего получить.
	if _, err := pool.NextID(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < latency/2 {
		t.Fatalf("expected first call to wait for network latency ~%v, got %v (test setup invalid)", latency, elapsed)
	}
}

// waitUntilAtLeast ждёт, пока fetcher.calls не станет >= want, либо не
// истечёт timeout.
func waitUntilAtLeast(t *testing.T, f *fakeFetcher, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f.callCount() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("fetcher was not called at least %d times within %v (got %d)", want, timeout, f.callCount())
}

// Close должен останавливать фоновую предвыборку — после него число
// вызовов fetcher больше не растёт.
func TestPool_Close_StopsBackgroundPrefetch(t *testing.T) {
	const batchLen = 5
	fetcher := &fakeFetcher{batchLen: batchLen, latency: time.Millisecond}
	pool := NewPool(fetcher, 3, time.Second)

	ctx := context.Background()
	if _, err := pool.NextID(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Даём Producer'у пару циклов на заполнение буфера, затем останавливаем.
	time.Sleep(30 * time.Millisecond)
	pool.Close()

	callsAfterClose := fetcher.callCount()
	time.Sleep(100 * time.Millisecond)

	if got := fetcher.callCount(); got != callsAfterClose {
		t.Fatalf("expected no further fetches after Close, before=%d after=%d", callsAfterClose, got)
	}
}

// Проверка потокобезопасности: много горутин одновременно запрашивают id,
// ни один id не выдаётся дважды. Гоняйте с -race.
func TestPool_ConcurrentNextID_NoDuplicates(t *testing.T) {
	const batchLen = 20
	fetcher := &fakeFetcher{batchLen: batchLen, latency: time.Millisecond}
	pool := NewPool(fetcher, 3, time.Second)
	defer pool.Close()

	ctx := context.Background()
	const goroutines = 10
	const perGoroutine = 20
	total := goroutines * perGoroutine

	results := make(chan uint64, total)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id, err := pool.NextID(ctx)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				results <- id
			}
		}()
	}
	wg.Wait()
	close(results)

	var count int64
	seen := make(map[uint64]bool, total)
	var mu sync.Mutex
	for id := range results {
		mu.Lock()
		if seen[id] {
			t.Fatalf("duplicate id %d returned under concurrent access", id)
		}
		seen[id] = true
		mu.Unlock()
		atomic.AddInt64(&count, 1)
	}
	if int(count) != total {
		t.Fatalf("expected %d ids total, got %d", total, count)
	}
}
