package idgenerator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// mockBatchGenerator имитирует источник генерации ID
type mockBatchGenerator struct {
	nextBatch IDBatch
	mockErr   error
	calls     int
}

func (m *mockBatchGenerator) NextBatch(size int) (IDBatch, error) {
	m.calls++
	if m.mockErr != nil {
		return nil, m.mockErr
	}
	return m.nextBatch, nil
}

// mockProducerBuffer имитирует буфер хранения батчей
type mockProducerBuffer struct {
	pushedBatches []IDBatch
	mockPushErr   error
}

func (m *mockProducerBuffer) Push(ctx context.Context, batch IDBatch) error {
	if m.mockPushErr != nil {
		return m.mockPushErr
	}
	m.pushedBatches = append(m.pushedBatches, batch)
	return nil
}

func (m *mockProducerBuffer) TakeBatch(ctx context.Context) (IDBatch, error) {
	return nil, nil // Не используется в тестах продюсера
}

// TestProducer_Run_Success проверяет один успешный цикл генерации и отправки в буфер
func TestProducer_Run_Success(t *testing.T) {
	// Создаем контекст, который автоматически отменится, чтобы выйти из бесконечного цикла
	ctx, cancel := context.WithCancel(context.Background())

	expectedBatch := IDBatch{10, 20, 30}
	mockGen := &mockBatchGenerator{nextBatch: expectedBatch}
	mockBuf := &mockProducerBuffer{}

	p := NewProducer(mockGen, mockBuf, 3)

	// Запускаем отмену контекста через короткое время, чтобы прервать цикл после первой итерации
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := p.Run(ctx)

	// Ожидаем ошибку отмены контекста как легитимный выход из цикла
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Expected context cancellation output, received: %v", err)
	}

	// Проверяем, что генератор был вызван и данные дошли до буфера
	if mockGen.calls == 0 {
		t.Fatal("The producer did not call the NextBatch generator.")
	}
	if len(mockBuf.pushedBatches) == 0 {
		t.Fatal("The batch was not delivered to the buffer")
	}
	if !reflect.DeepEqual(mockBuf.pushedBatches[0], expectedBatch) {
		t.Errorf("An invalid batch was delivered to the buffer.\nExpected: %v\nReceived: %v", expectedBatch, mockBuf.pushedBatches[0])
	}
}

// TestProducer_Run_GeneratorError проверяет реакцию на ошибку генератора.
// Внимание: Согласно вашему текущему коду, при ошибке генератора (if batchErr != nil)
// происходит мгновенный выход (return batchErr). Этот тест проверяет именно это поведение.
func TestProducer_Run_GeneratorError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("critical counter database error")

	mockGen := &mockBatchGenerator{mockErr: expectedErr}
	mockBuf := &mockProducerBuffer{}

	p := NewProducer(mockGen, mockBuf, 5)

	// Код должен упасть на первой же итерации и вернуть ошибку генератора
	err := p.Run(ctx)

	if err == nil {
		t.Fatal("A generator error was expected, but the Run method returned nil.")
	}
	if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
		t.Errorf("An invalid error was received.\nExpected: %v\nReceived: %v", expectedErr, err)
	}

	// Гарантируем, что в буфер ничего не отправлялось
	if len(mockBuf.pushedBatches) > 0 {
		t.Error("Error! The producer sent data to the buffer despite the generator failure.")
	}
}

// TestProducer_Run_PanicRecovery проверяет, что паника внутри цикла перехватывается дефером
func TestProducer_Run_PanicRecovery(t *testing.T) {
	ctx := context.Background()

	mockGen := &mockBatchGenerator{} // Оставим nil, вызов NextBatch на nil-структуре вызовет панику
	mockGen = nil                    // Принудительно провоцируем panic: runtime error внутри Run

	mockBuf := &mockProducerBuffer{}
	p := NewProducer(mockGen, mockBuf, 2)

	err := p.Run(ctx)

	if err == nil {
		t.Fatal("A caught panic error was expected, but the Run method returned nil")
	}

	// Проверяем, что в ошибке есть маркер паники из вашего defer
	expectedPrefix := "producer panic:"
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:15] != expectedPrefix {
		t.Errorf("Invalid panic error format. Received: %v", err)
	}
}
