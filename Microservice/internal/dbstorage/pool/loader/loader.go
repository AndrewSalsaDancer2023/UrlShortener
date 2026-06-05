package loader

import (
	"context"
	"errors"
	"fmt"
	"time"
	"urlshortener/internal/dbstorage/config"
	"urlshortener/internal/dbstorage/pool"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LoadURLPool struct {
	pool *pgxpool.Pool
}

func New(conf *config.DBConfig) (pool.DBGetURLPool, error) {

	pool, err := pool.CreatePool(conf)
	if err != nil {
		return nil, err
	}

	return &LoadURLPool{
		pool: pool,
	}, nil
}

func (loader *LoadURLPool) Load(ctx context.Context, shortURL int64) (string, error) {
	var longURL string
	query := "SELECT long_url FROM short_urls WHERE short_url = $1"

	err := loader.pool.QueryRow(ctx, query, shortURL).Scan(&longURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("error during getting long URL: %w", err)
		}
		return "", err
	}

	return longURL, nil
}

func (loader *LoadURLPool) TryConnect() error {
	// Создаем контекст с таймаутом в 5 секунд на базе пустого Background-контекста
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := loader.pool.Ping(ctx); err != nil {
		loader.pool.Close()
		return err
	}

	return nil
}

func (loader *LoadURLPool) Close() {
	loader.pool.Close()
}
