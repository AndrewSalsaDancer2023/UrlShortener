package pool

import (
	"context"
	"time"
	"urlshortener/internal/dbstorage/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBSaveURLPool interface {
	Save(ctx context.Context, shortURL int64, longURL string) (int64, error)
}

type DBGetURLPool interface {
	Load(ctx context.Context, shortURL int64) (string, error)
}

func CreatePool(ctx context.Context, conf *config.DBConfig) (*pgxpool.Pool, error) {

	config, err := pgxpool.ParseConfig(conf.Dsn)
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
	return pool, nil
}
