package idgenerator

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "urlshortener/internal/proto/idservice"

	"google.golang.org/grpc"
)

// mockBatchFetcher — фейковый gRPC-клиент для симуляции ответов сервера
// mockBatchFetcher — фейковый gRPC-клиент для симуляции ответов сервера
type mockBatchFetcher struct {
	mu            sync.Mutex
	nextIDs       []int64 // Для фиксированных ответов (первый тест)
	nextIDCounter int64   // Для динамической генерации (тест конкурентности)
	mockError     error
}

func (m *mockBatchFetcher) GetIDBatch(ctx context.Context, in *pb.GetBatchRequest, opts ...grpc.CallOption) (*pb.GetBatchResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mockError != nil {
		return nil, m.mockError
	}

	// Сценарий 1: Если в тесте жестко задана пачка ID (например, {100, 101, 102})
	if len(m.nextIDs) > 0 {
		resp := &pb.GetBatchResponse{Ids: m.nextIDs}
		return resp, nil
	}

	// Сценарий 2: Если пачка не задана, генерируем инкрементальные уникальные ID
	batchSize := 5
	ids := make([]int64, batchSize)
	for i := 0; i < batchSize; i++ {
		m.nextIDCounter++
		ids[i] = m.nextIDCounter
	}

	return &pb.GetBatchResponse{Ids: ids}, nil
}

// TestPool_Success проверяет базовый сценарий: предвыборку и успешную выдачу ID по очереди
func TestPool_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mockClient := &mockBatchFetcher{
		nextIDs: []int64{100, 101, 102},
	}

	var wg sync.WaitGroup
	// Инициализируем пул с глубиной очереди 1 батч
	pool := NewPool(ctx, mockClient, 1, 100*time.Millisecond, &wg)
	defer func() {
		pool.Close()
		wg.Wait() // Гарантируем чистое завершение горутины продюсера
	}()

	// Запрашиваем элементы один за другим.
	// Пул должен прозрачно выдать элементы из первого батча {100, 101, 102}
	expectedIDs := []int64{100, 101, 102}
	for _, expected := range expectedIDs {
		id, err := pool.NextID(ctx)
		if err != nil {
			t.Fatalf("Не ожидали ошибку при вызове NextID: %v", err)
		}
		if id != expected {
			t.Errorf("Получен некорректный ID. Ожидалось: %d, Получено: %d", expected, id)
		}
	}
}

// TestPool_ConcurrentAccess проверяет потокобезопасность пула (Race Conditions)
// при одновременном обращении сотен горутин
func TestPool_ConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Инициализируем мок со счетчиком, стартующим с 0
	mockClient := &mockBatchFetcher{
		nextIDCounter: 0,
	}

	var wg sync.WaitGroup
	pool := NewPool(ctx, mockClient, 2, 500*time.Millisecond, &wg)
	defer func() {
		pool.Close()
		wg.Wait()
	}()

	const goroutinesCount = 50
	resultsChan := make(chan int64, goroutinesCount)

	var workersWg sync.WaitGroup
	for i := 0; i < goroutinesCount; i++ {
		workersWg.Add(1)
		go func() {
			defer workersWg.Done()
			id, err := pool.NextID(ctx)
			if err == nil {
				resultsChan <- id
			}
		}()
	}

	workersWg.Wait()
	close(resultsChan)

	uniqueIDs := make(map[int64]bool)
	for id := range resultsChan {
		if uniqueIDs[id] {
			t.Errorf("Обнаружен дубликат ID: %d! Пул не потокобезопасен.", id)
		}
		uniqueIDs[id] = true
	}

	// Опционально: проверим, что мы действительно собрали ровно 50 уникальных ID
	if len(uniqueIDs) != goroutinesCount {
		t.Errorf("Ожидалось %d уникальных ID, получено %d", goroutinesCount, len(uniqueIDs))
	}
}
