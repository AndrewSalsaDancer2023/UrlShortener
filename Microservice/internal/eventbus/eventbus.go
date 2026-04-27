package eventbus

import (
	"fmt"
	"log"
	"time"

	eventbus "github.com/asaskevich/eventbus"
)

// Имена топиков — константы, чтобы не опечататься в строках.
const (
	TopicIDGenerated = "id.generated"
	TopicIDError     = "id.error"
)

// IDGeneratedEvent публикуется при каждой успешной генерации ID.
type IDGeneratedEvent struct {
	NumericID int64
	ShortCode string
	Timestamp time.Time
}

// IDErrorEvent публикуется при ошибке генерации.
type IDErrorEvent struct {
	Err       error
	Timestamp time.Time
}

// Bus — обёртка над asaskevich/eventbus с типизированными методами.
type Bus struct {
	bus eventbus.Bus
}

func New() *Bus {
	return &Bus{bus: eventbus.New()}
}

// PublishIDGenerated публикует событие успешной генерации.
func (b *Bus) PublishIDGenerated(e IDGeneratedEvent) {
	b.bus.Publish(TopicIDGenerated, e)
}

// PublishIDError публикует событие ошибки.
func (b *Bus) PublishIDError(e IDErrorEvent) {
	b.bus.Publish(TopicIDError, e)
}

// SubscribeIDGenerated подписывается на успешные генерации.
func (b *Bus) SubscribeIDGenerated(fn func(IDGeneratedEvent)) error {
	return b.bus.Subscribe(TopicIDGenerated, fn)
}

// SubscribeIDError подписывается на ошибки генерации.
func (b *Bus) SubscribeIDError(fn func(IDErrorEvent)) error {
	return b.bus.Subscribe(TopicIDError, fn)
}

// RegisterDefaultSubscribers регистрирует стандартные подписчики:
// аудит-лог и счётчик ошибок.
func RegisterDefaultSubscribers(b *Bus) {
	// Подписчик 1: аудит — логируем каждый выданный ID
	if err := b.SubscribeIDGenerated(func(e IDGeneratedEvent) {
		log.Printf("[AUDIT] id=%d short=%s ts=%s",
			e.NumericID, e.ShortCode, e.Timestamp.Format(time.RFC3339Nano))
	}); err != nil {
		log.Printf("[WARN] failed to subscribe audit logger: %v", err)
	}

	// Подписчик 2: метрики — простой счётчик в stdout (в реальном проекте — Prometheus)
	if err := b.SubscribeIDGenerated(func(e IDGeneratedEvent) {
		fmt.Printf("[METRICS] generated_ids_total+1\n")
	}); err != nil {
		log.Printf("[WARN] failed to subscribe metrics: %v", err)
	}

	// Подписчик 3: логирование ошибок
	if err := b.SubscribeIDError(func(e IDErrorEvent) {
		log.Printf("[ERROR] id generation failed: %v at %s",
			e.Err, e.Timestamp.Format(time.RFC3339Nano))
	}); err != nil {
		log.Printf("[WARN] failed to subscribe error logger: %v", err)
	}
}
