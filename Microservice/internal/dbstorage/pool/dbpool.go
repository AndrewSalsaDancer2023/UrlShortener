package pool

import (
	"context"
	"time"
	"urlshortener/internal/dbstorage/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBTime interface {
	Now() time.Time
}

type DBTimeReal struct{}

func (DBTimeReal) Now() time.Time {
	return time.Now().UTC() // Рекомендуется всегда работать в UTC
}

type CommonDBPool interface {
	TryConnect() error
	Close()
}

type DBSaveURLPool interface {
	Save(context.Context, int64, string, DBTime) (int64, error)
	CommonDBPool
}

type DBGetURLPool interface {
	Load(context.Context, int64) (string, error)
	CommonDBPool
}

func CreatePoolConfig(dbConf *config.DBConfig, poolConf *pgxpool.Config) {
	poolConf.MinConns = dbConf.MinConns
	poolConf.MaxConns = dbConf.MaxConns
	poolConf.MaxConnIdleTime = dbConf.MaxConnIdleTime
	poolConf.ConnConfig.ConnectTimeout = dbConf.MaxConnectTimeout
}

func CreatePool(conf *config.DBConfig) (*pgxpool.Pool, error) {

	config, err := pgxpool.ParseConfig(conf.Dsn)
	if err != nil {
		return nil, err
	}
	/*
		config.MaxConns = conf.MaxConns
		config.MinConns = conf.MinConns

		config.MaxConnIdleTime = conf.MaxConnIdleTime
		config.MaxConnLifetime = conf.MaxConnLifetime

		config.ConnConfig.ConnectTimeout = 5 * time.Second
	*/
	CreatePoolConfig(conf, config)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
