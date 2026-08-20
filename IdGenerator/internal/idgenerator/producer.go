// Package producer владеет циклом оркестрации между генерацией ID
// (интерфейс Generator, объявленный ниже) и очередью готовых батчей
// (buffer.Buffer).
//
// Ни Generator, ни Buffer не знают друг о друге — Producer единственный,
// кто их связывает. Это позволяет тестировать и переиспользовать
// Generator и Buffer независимо друг от друга (см. тесты в internal/buffer
// и internal/idgen — ни один из них не упоминает второй пакет).
package idgenerator

import (
	"context"
	"fmt"
	"log"
	// "idgen-service/internal/buffer"
	// "idgen-service/internal/metrics"
)

// Generator описывает то, что нужно Producer'у от источника ID.
// Определён здесь, а не в пакете idgen — по правилу Go "принимай
// интерфейсы, возвращай структуры": контракт формулирует потребитель,
// а не поставщик. idgen.CounterGenerator удовлетворяет этому интерфейсу
// неявно, просто имея метод с нужной сигнатурой — явная реализация
// (implements) в Go не нужна и не объявляется.
//
// Контракт: NextBatch либо возвращает СТРОГО n элементов и nil error,
// либо (nil, error) — реализация никогда не должна молча возвращать
// батч меньшего размера без ошибки. Producer ниже полагается именно
// на этот контракт при принятии решения о retry.

type Producer struct {
	gen BatchGenerator
	// buf       *Buffer
	buf       IDBuffer
	batchSize int
}

// New создаёт Producer, который будет генерировать батчи размером
// batchSize через gen и складывать их в buf.
func NewProducer(gen BatchGenerator, buf IDBuffer, batchSize int) *Producer {
	p := &Producer{
		gen:       gen,
		buf:       buf,
		batchSize: batchSize,
	}

	return p
}

// Run — бесконечный цикл: сгенерировать батч -> положить в буфер.
//
// Если Generator.NextBatch вернул ошибку, Producer НЕ передаёт частичные
// или пустые данные дальше (Buffer.Push для них не вызывается вообще) —
// вместо этого выжидается интервал экспоненциального backoff (сбрасывается
// до initialBackoff после первого же успеха) и генерация повторяется.
// Это разумный выбор по умолчанию для транзиентных ошибок (сетевой сбой,
// временная недоступность внешнего сервиса ID и т.п.).
//
// Если ошибка НЕ транзиентная (например, окончательно исчерпано
// пространство значений счётчика), Run продолжит ретраить бесконечно —
// это осознанный компромисс простоты. Если для вашего Generator нужно
// разделять "стоит ретраить" и "нужно остановить сервис насовсем",
// проверяйте тип ошибки через errors.Is/errors.As внутри Run и вызывайте
// return при фатальных ошибках (сигнализируя об этом наружу, например,
// через отдельный канал, на который main.go подписан для перевода
// health-check в NOT_SERVING).
//
// Push сам блокируется, если буфер полон, и сам же реагирует на
// отмену ctx — Producer.Run просто транслирует эту отмену в выход
// из цикла.
//
// Предназначен для запуска в отдельной горутине: go p.Run(ctx).
func (p *Producer) Run(ctx context.Context) (err error) {

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[CRITICAL] Producer run fall with panic: %v.", r)
			err = fmt.Errorf("producer panic: %v", r)
		}
	}()

	for {
		// Проверяем, не завершен ли контекст перед генерацией новой пачки
		if err = ctx.Err(); err != nil {
			log.Printf("Shutting down Run  with error: %v", err)
			return err
		}

		batch, batchErr := p.gen.NextBatch(p.batchSize)
		if batchErr != nil {
			// metrics.GenerationErrors.Inc()
			return batchErr

		}
		//log.Println("Generated next batch")
		if pushErr := p.buf.Push(ctx, batch); pushErr != nil {
			// ctx отменён/просрочен, пока Push ждал место в буфере —
			// корректно завершаем горутину, ничего не "теряя" молча.
			return fmt.Errorf("buffer push failed: %w", pushErr)
		}
	}
}
