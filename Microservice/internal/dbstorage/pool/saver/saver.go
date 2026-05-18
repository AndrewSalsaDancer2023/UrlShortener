package saver

import (
	"context"
	"fmt"
	"urlshortener/internal/dbstorage/config"
	"urlshortener/internal/dbstorage/pool"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SaveURLPool struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, conf *config.DBConfig) (pool.DBSaveURLPool, error) {
	/*
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}

		config.MaxConns = conf.MaxConns
		config.MinConns = conf.MinConns

		config.MaxConnIdleTime = conf.MaxConnIdleTime
		config.MaxConnLifetime = conf.MaxConnLifetime

		config.ConnConfig.ConnectTimeout = 5 * time.Second

		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			return nil, err
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	*/
	pool, err := pool.CreatePool(ctx, conf)
	if err != nil {
		return nil, err
	}

	return &SaveURLPool{
		pool: pool,
	}, nil
}

func (saver *SaveURLPool) Save(ctx context.Context, shortURL int64, longURL string) (int64, error) {
	query := `
                INSERT INTO short_urls (short_url, long_url) 
                VALUES ($1, $2)
                ON CONFLICT (long_url) 
                DO UPDATE SET long_url = EXCLUDED.long_url
                RETURNING id;
        `

	var finalID int64
	// pgx автоматически использует Prepared Statements под капотом для ускорения
	err := saver.pool.QueryRow(ctx, query, shortURL, longURL).Scan(&finalID)
	if err != nil {
		return 0, fmt.Errorf("url pair insert error: %w", err)
	}

	return finalID, nil
}
