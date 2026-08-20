// реализует ограниченную потокобезопасную очередь готовых
// батчей ID.
//
// Buffer НИЧЕГО не знает о том, как эти батчи генерируются — это
// сознательное разделение ответственности: Buffer отвечает только за
// хранение и backpressure (блокировки при полном/пустом состоянии),
// а генерация ID — забота отдельного Generator, оркестрацию между
// ними ведёт Producer (см. internal/producer).
//
// Внутри канал хранит НЕ отдельные ID, а уже готовые батчи ([]uint64).
// Одна операция с каналом (Push/TakeBatch) двигает сразу весь батч —
// это даёт кратно меньше блокировок рантайма Go по сравнению со
// схемой "канал из отдельных uint64 + N поштучных receive на батч".
package idgenerator

import (
	"context"
)

// IDBuffer описывает методы, которые нужны обработчику
type IDBuffer interface {
	Push(ctx context.Context, batch IDBatch) error
	TakeBatch(ctx context.Context) (IDBatch, error)
}

type Buffer struct {
	ch chan IDBatch
	IDBuffer
}

// NewBuffer создаёт буфер напрямую по числу готовых
// батчей. Нам нужно иметь несколько готовых батчей,
// например, в client.Pool
func NewBuffer(batches int) IDBuffer {
	//	slots := bufferSize / batchSize
	if batches < 1 {
		batches = 1
	}
	// metrics.BufferCapacity.Set(float64(slots))

	return &Buffer{ch: make(chan IDBatch, batches)}
}

// Push помещает готовый батч в буфер. Блокируется, если все слоты заняты,
// пока consumer не освободит место (TakeBatch) либо не отменится контекст ctx.
func (b *Buffer) Push(ctx context.Context, batch IDBatch) error {
	// start := time.Now()
	select {
	case b.ch <- batch:
		// if blocked := time.Since(start); blocked > time.Millisecond {
		// 	metrics.ProducerBlockedSeconds.Observe(blocked.Seconds())
		// }
		// metrics.BatchesProduced.Inc()
		// metrics.BufferLevel.Set(float64(len(b.ch)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TakeBatch извлекает один готовый батч. Блокируется, если буфер пуст,
// пока producer не положит новый батч (Push) либо не отменится ctx.
func (b *Buffer) TakeBatch(ctx context.Context) (IDBatch, error) {
	// start := time.Now()
	select {
	case batch := <-b.ch:
		// if blocked := time.Since(start); blocked > time.Millisecond {
		// 	metrics.ConsumerBlockedSeconds.Observe(blocked.Seconds())
		// }
		// metrics.BatchesConsumed.Inc()
		// metrics.BufferLevel.Set(float64(len(b.ch)))
		return batch, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
