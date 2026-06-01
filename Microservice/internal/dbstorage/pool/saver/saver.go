package saver

import (
	"context"
	"fmt"
	"time"
	"urlshortener/internal/dbstorage/config"
	"urlshortener/internal/dbstorage/pool"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SaveURLPool struct {
	pool *pgxpool.Pool
}

func New(conf *config.DBConfig) (pool.DBSaveURLPool, error) {

	pool, err := pool.CreatePool(conf)
	if err != nil {
		return nil, err
	}

	return &SaveURLPool{
		pool: pool,
	}, nil
}

func (saver *SaveURLPool) TryConnect() error {
	// Создаем контекст с таймаутом в 5 секунд на базе пустого Background-контекста
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := saver.pool.Ping(ctx); err != nil {
		saver.pool.Close()
		return err
	}

	return nil
}

func (saver *SaveURLPool) Close() {
	saver.pool.Close()
}

func (saver *SaveURLPool) Save(ctx context.Context, shortURL int64, longURL string, timer pool.DBTime) (int64, error) {
	query := `
                INSERT INTO short_urls (short_url, long_url, created_at) 
                VALUES ($1, $2, $3)
                ON CONFLICT (long_url) 
                DO UPDATE SET long_url = EXCLUDED.long_url
                RETURNING short_url;
        `

	var savedShortURL int64
	creationTime := timer.Now()
	err := saver.pool.QueryRow(ctx, query, shortURL, longURL, creationTime).Scan(&savedShortURL)
	if err != nil {
		return 0, fmt.Errorf("url pair insert error: %w", err)
	}

	return savedShortURL, nil
}
