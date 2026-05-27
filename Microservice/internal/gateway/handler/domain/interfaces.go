package domain

import (
	"context"
)

// GenerateResponse — ответ микросервиса генерации ID.
type GenerateResponse struct {
	NumericID int64  `json:"numeric_id"`
	ShortCode string `json:"short_code"`
}

// IDGeneratorInterface — интерфейс для подмены клиента в тестах.
type IDGeneratorInterface interface {
	Generate(context.Context) (*GenerateResponse, error)
}

type URLShortenerInterface interface {
	Shorten(context.Context, int64, string) (int64, error)
}

type URLRestorerInterface interface {
	Restore(context.Context, int64) (string, error)
}

type URLCacheInterface interface {
	SaveURLPair(context.Context, int64, string) error //(int64, error)
	GetShortURL(context.Context, int64, string) (int64, error)
	GetLongURL(context.Context, int64) (string, error)
}
