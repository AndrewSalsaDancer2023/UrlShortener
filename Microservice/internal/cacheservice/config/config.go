package config

import (
	"time"
	"urlshortener/internal/dbstorage/config"
)

// содержит параметры пула подключений к базе данных
// Все значения читаются из переменных окружения или из флагов
type CacheConfig struct {
	//порт для подключения сервиса
	Port string
	//хост сервера
	Host string

	// PoolSizeMult         int
	MinIdleConnsMult int
	MaxIdleConnsMult int
	ConnMaxIdleTime  time.Duration
}

func GetURLCacheConfig() CacheConfig {
	return CacheConfig{
		Port: config.GetEnv("PORT", "6379"),
		Host: config.GetEnv("HOST", "127.0.0.1"),
		// PoolSizeMult:         config.GetEnvInt("POOLSIZE", 10),
		MinIdleConnsMult: config.GetEnvInt("MINIDDLECONNS", 2),
		MaxIdleConnsMult: config.GetEnvInt("MAXIDDLECONNS", 10),
		ConnMaxIdleTime:  config.GetEnvDuration("MAXIDDLETIME", 5*time.Minute),
	}
}
