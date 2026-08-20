package idgenerator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// TestBuffer_PushAndTakeBatch_Success проверяет штатный сценарий записи и чтения
func TestBuffer_PushAndTakeBatch_Success(t *testing.T) {
	ctx := context.Background()
	buf := NewBuffer(2) // Создаем буфер емкостью 2 батча

	testBatch1 := IDBatch{1, 2, 3}
	testBatch2 := IDBatch{4, 5, 6}

	// Записываем батчи в буфер
	if err := buf.Push(ctx, testBatch1); err != nil {
		t.Fatalf("Не удалось записать первый батч: %v", err)
	}
	if err := buf.Push(ctx, testBatch2); err != nil {
		t.Fatalf("Не удалось записать второй батч: %v", err)
	}

	// Читаем первый батч
	gotBatch1, err := buf.TakeBatch(ctx)
	if err != nil {
		t.Fatalf("Не удалось прочитать первый батч: %v", err)
	}
	if !reflect.DeepEqual(gotBatch1, testBatch1) {
		t.Errorf("Первый батч поврежден. Ожидалось: %v, Получено: %v", testBatch1, gotBatch1)
	}

	// Читаем второй батч
	gotBatch2, err := buf.TakeBatch(ctx)
	if err != nil {
		t.Fatalf("Failed to read second batch: %v", err)
	}
	if !reflect.DeepEqual(gotBatch2, testBatch2) {
		t.Errorf("The second batch is corrupted. Expected: %v, Received: %v", testBatch2, gotBatch2)
	}
}

// TestBuffer_TakeBatch_ContextCanceled проверяет разблокировку пустого буфера при отмене контекста
func TestBuffer_TakeBatch_ContextCanceled(t *testing.T) {
	// Создаем контекст, который мы отменим вручную
	ctx, cancel := context.WithCancel(context.Background())
	buf := NewBuffer(1)

	// Запускаем отмену контекста через короткий промежуток времени в фоне
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	// Пытаемся прочитать из пустого буфера. Метод должен заблокироваться,
	// а после фоновой отмены контекста — мгновенно вернуть ошибку.
	_, err := buf.TakeBatch(ctx)
	if err == nil {
		t.Fatal("Expected context cancellation error, but TakeBatch returned nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected error %v, received: %v", context.Canceled, err)
	}
}

// TestBuffer_Push_ContextDeadlineExceeded проверяет поведение при переполнении буфера
func TestBuffer_Push_ContextDeadlineExceeded(t *testing.T) {
	// Создаем контекст с коротким таймаутом в 50 миллисекунд
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	buf := NewBuffer(1) // Буфер вмещает только 1 элемент
	testBatch := IDBatch{10, 20}

	// Заполняем буфер до предела
	if err := buf.Push(ctx, testBatch); err != nil {
		t.Fatalf("The first push should have been successful, but returned an error: %v", err)
	}

	// Пытаемся записать второй элемент в полный буфер.
	// Метод заблокируется и через 50мс должен отвалиться по таймауту.
	err := buf.Push(ctx, IDBatch{30, 40})
	if err == nil {
		t.Fatal("A timeout error was expected, but Push returned nil.")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected error %v, received: %v", context.DeadlineExceeded, err)
	}
}
