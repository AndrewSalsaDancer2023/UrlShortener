package config

import (
	"os"
	"strconv"
	"time"
)

// содержит параметры пула подключений к базе данных
// Все значения читаются из переменных окружения или из флагов
type DBConfig struct {
	//порт для подключения сервиса
	Port string
	//dsn для базы
	Dsn string
	// минимальный размер пула
	MinConns int32

	// максимальный размер пула
	MaxConns int32

	// максимальное время жизни соединения
	//MaxConnLifetime time.Duration

	// максимальное время на установку одного физического TCP-соединения
	MaxConnectTimeout time.Duration

	// максимальное время простоя соединения
	MaxConnIdleTime time.Duration
	//таймаут на чтение
	ReadTimeout time.Duration
	//таймаут на запись
	WriteTimeout time.Duration
}

func GetEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func GetEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func GetEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
