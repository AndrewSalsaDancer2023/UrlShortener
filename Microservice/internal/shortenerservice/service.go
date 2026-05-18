package shortenerservice

import (
	"context"
	"fmt"
	"urlshortener/internal/dbstorage/pool"
)

// IDService — сервис генерации идентификаторов.
// Зависимости внедряются через конструктор (Constructor Injection).
type ShortenerService struct {
	pool pool.DBSaveURLPool
}

// New создаёт IDService с внедрёнными зависимостями.
func New(savepool pool.DBSaveURLPool) *ShortenerService {
	return &ShortenerService{
		pool: savepool,
	}
}

// Generate генерирует новый уникальный идентификатор и публикует событие.
func (s *ShortenerService) SaveURLPair(ctx context.Context, short_url int64, long_url string) (int64, error) {

	url_id, err := s.pool.Save(ctx, short_url, long_url)

	if err != nil {
		return 0, fmt.Errorf("failed to encode id: %w", err)
	}

	return url_id, nil
}
