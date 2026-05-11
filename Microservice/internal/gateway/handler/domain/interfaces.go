package domain

import (
	"context"
)

// GenerateResponse — ответ микросервиса генерации ID.
type GenerateResponse struct {
	NumericID int64  `json:"numeric_id"`
	ShortCode string `json:"short_code"`
}

// Doer — интерфейс для подмены клиента в тестах.
type Doer interface {
	Generate(ctx context.Context) (*GenerateResponse, error)
}
