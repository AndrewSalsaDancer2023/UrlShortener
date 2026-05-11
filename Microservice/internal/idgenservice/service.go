package grpcservice

import (
	"fmt"

	"urlshortener/internal/base62"
	"urlshortener/internal/generator"
)

// GenerateResult содержит результат генерации.
type GenerateResult struct {
	NumericID int64  `json:"numeric_id"` // исходный Snowflake ID
	ShortCode string `json:"short_code"` // Base62-представление
}

// IDService — сервис генерации идентификаторов.
// Зависимости внедряются через конструктор (Constructor Injection).
type IDService struct {
	gen     generator.IDGenerator
	encoder base62.Encoder
}

// New создаёт IDService с внедрёнными зависимостями.
func New(gen generator.IDGenerator, encoder base62.Encoder /*, bus *eventbus.Bus*/) *IDService {
	return &IDService{
		gen:     gen,
		encoder: encoder,
	}
}

// Generate генерирует новый уникальный идентификатор и публикует событие.
func (s *IDService) Generate() (*GenerateResult, error) {
	id, err := s.gen.NextID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate id: %w", err)
	}

	short, err := s.encoder.Encode(id)
	if err != nil {
		return nil, fmt.Errorf("failed to encode id: %w", err)
	}

	result := &GenerateResult{
		NumericID: id,
		ShortCode: short,
	}
	/*
		s.bus.PublishIDGenerated(eventbus.IDGeneratedEvent{
			NumericID: id,
			ShortCode: short,
			Timestamp: time.Now(),
		})
	*/
	return result, nil
}
