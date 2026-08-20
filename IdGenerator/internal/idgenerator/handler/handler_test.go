package handler

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"urlshortener/internal/idgenerator"
	pb "urlshortener/internal/proto/idservice"
)

// mockIDBuffer имитирует поведение idgenerator.IDBuffer в тестах
type mockIDBuffer struct {
	nextBatch []int64
	mockErr   error
}

// Push — реализуем обязательный метод интерфейса для компиляции теста.
// В хэндлере он не вызывается, поэтому возвращаем nil.
func (m *mockIDBuffer) Push(ctx context.Context, batch idgenerator.IDBatch) error {
	return nil
}

// TakeBatch — реализуем чтение (используется gRPC хэндлером)
func (m *mockIDBuffer) TakeBatch(ctx context.Context) (idgenerator.IDBatch, error) {
	if m.mockErr != nil {
		return nil, m.mockErr
	}
	return m.nextBatch, nil
}

// TestGRPCHandler_GetIDBatch_Success проверяет успешное получение пачки ID из буфера
func TestGRPCHandler_GetIDBatch_Success(t *testing.T) {
	ctx := context.Background()

	// 1. Готовим тестовые данные
	expectedBatch := []int64{500, 501, 502}
	mockBuf := &mockIDBuffer{
		nextBatch: expectedBatch,
	}

	// 2. Инициализируем хэндлер с фейковым буфером
	handler := NewHandler(mockBuf)

	// 3. Вызываем тестируемый метод напрямую
	resp, err := handler.GetIDBatch(ctx, &pb.GetBatchRequest{})

	// 4. Проверяем результаты
	if err != nil {
		t.Fatalf("Unexpected error during GetIDBatch call: %v", err)
	}

	if resp == nil {
		t.Fatal("Getting nil instead of an answer GetBatchResponse")
	}

	if !reflect.DeepEqual(resp.Ids, expectedBatch) {
		t.Errorf("Invalid batch ID received.\nExpected: %v\n Received: %v", expectedBatch, resp.Ids)
	}
}

// TestGRPCHandler_GetIDBatch_BufferError проверяет реакцию хэндлера на ошибку внутри буфера
func TestGRPCHandler_GetIDBatch_BufferError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Симулируем ситуацию, когда буфер пуст и чтение прерывается по таймауту контекста
	expectedErr := context.DeadlineExceeded
	mockBuf := &mockIDBuffer{
		mockErr: expectedErr,
	}

	handler := NewHandler(mockBuf)

	// Вызываем метод
	resp, err := handler.GetIDBatch(ctx, &pb.GetBatchRequest{})

	// Проверяем, что ошибка корректно проброшена наружу
	if resp != nil {
		t.Errorf("Expected nil in response on buffer error, but received: %v", resp)
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Received the wrong error.\nExpected: %v\nReceived: %v", expectedErr, err)
	}
}
