package utils

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// SafeErrorBuffer инкапсулирует безопасную работу со срезом ошибок
type SafeErrorBuffer struct {
	mu   sync.Mutex
	errs []error
}

// Append добавляет ошибку в срез, блокируя доступ для других горутин на время записи
func (sb *SafeErrorBuffer) Append(err error) {
	if err == nil {
		return
	}
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.errs = append(sb.errs, err)
}

// Get возвращает все собранные ошибки
func (sb *SafeErrorBuffer) GetErrors() []error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.errs
}

func ShutdownResourceParallel(ctx context.Context, wg *sync.WaitGroup, errBuffer *SafeErrorBuffer,
	resourceName string, closeFunc func() error) {
	wg.Add(1) // Синхронно увеличиваем счетчик в вызывающем потоке

	go func() {
		defer wg.Done()

		done := make(chan struct{})
		var closeErr error

		go func() {
			closeErr = closeFunc()
			close(done)
		}()

		select {
		case <-done:
			if closeErr != nil {
				// Ошибка самой базы данных при закрытии
				wrappedErr := fmt.Errorf("closing error %s: %w", resourceName, closeErr)
				errBuffer.Append(wrappedErr)
			} else {
				log.Printf("resource %s closed succesfully.", resourceName)
			}
		case <-ctx.Done():
			// Ошибка таймаута
			timeoutErr := fmt.Errorf("resource %s didn't have time to close: %w", resourceName, ctx.Err())
			errBuffer.Append(timeoutErr)
		}
	}()
}
