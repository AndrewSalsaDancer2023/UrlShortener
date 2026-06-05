package config

import (
	"time"
)

func GetURLSaverConfig() DBConfig {
	return DBConfig{
		Port:              GetEnv("PORT", "50053"),
		Dsn:               GetEnv("DSN", "postgres://postgres:admin@localhost:5432/urls_db"),
		MinConns:          int32(GetEnvInt("MIN_CONNECTIONS", 5)),
		MaxConns:          int32(GetEnvInt("MAX_CONNECTIONS", 10)),
		MaxConnIdleTime:   GetEnvDuration("MAX_IDDLE_TIME", 5*time.Second),
		MaxConnectTimeout: GetEnvDuration("MAX_CONNECT_TIMEOUT", 3*time.Second),
		WriteTimeout:      GetEnvDuration("MAXWRITETIME", 300*time.Millisecond),
		// MaxConnLifetime: GetEnvDuration("CONN_LIFE_TIME", time.Hour),
	}
}

func GetURLReaderConfig() DBConfig {
	return DBConfig{
		Port:              GetEnv("PORT", "50054"),
		Dsn:               GetEnv("DSN", "postgres://postgres:admin@localhost:5432/urls_db"),
		MinConns:          int32(GetEnvInt("MIN_CONNECTIONS", 16)),
		MaxConns:          int32(GetEnvInt("MAX_CONNECTIONS", 32)),
		MaxConnIdleTime:   GetEnvDuration("MAX_IDDLE_TIME", 5*time.Second),
		MaxConnectTimeout: GetEnvDuration("MAX_CONNECT_TIMEOUT", 3*time.Second),
		ReadTimeout:       GetEnvDuration("MAXREADTIME", 50*time.Millisecond),
		// MaxConnLifetime: GetEnvDuration("CONN_LIFE_TIME", 30*time.Minute),
	}
}
