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

type Buffer struct {
	ch chan IDBatch
}

// NewBuffer создаёт буфер общей ёмкостью bufferSize элементов,
// хранимых батчами по batchSize штук. Ёмкость канала в батчах:
// bufferSize/batchSize (например, 1000/250 = 4 слота).
func NewBuffer(bufferSize, batchSize int) *Buffer {
	slots := bufferSize / batchSize
	if slots < 1 {
		slots = 1
	}
	// metrics.BufferCapacity.Set(float64(slots))

	return &Buffer{ch: make(chan IDBatch, slots)}
}

// NewBufferWithSlots создаёт буфер напрямую по числу слотов (готовых
// батчей), без пересчёта из bufferSize/batchSize. Полезно там, где
// единица "сколько батчей держать наготове" естественна сама по себе —
// например, в client.Pool, где реальный размер батча определяет
// удалённый gRPC-сервис, а не сам буфер.
func NewBufferWithSlots(slots int) *Buffer {
	if slots < 1 {
		slots = 1
	}
	// metrics.BufferCapacity.Set(float64(slots))

	return &Buffer{ch: make(chan IDBatch, slots)}
}

// Push кладёт готовый батч в буфер. Блокируется, если все слоты заняты,
// пока consumer не освободит место (TakeBatch) либо не отменится ctx.
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

// Level возвращает текущее число готовых батчей в буфере (для health/debug).
func (b *Buffer) Level() int { return len(b.ch) }

// Capacity возвращает ёмкость буфера в батчах.
func (b *Buffer) Capacity() int { return cap(b.ch) }
