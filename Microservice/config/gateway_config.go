package config

import (
	"time"
	"urlshortener/internal/dbstorage/config"
)

// GatewayConfig содержит параметры API Gateway.
// Все значения читаются из переменных окружения с fallback на дефолты.
type GatewayConfig struct {
	// Адрес самого Gateway
	Port string

	// Адрес upstream-микросервиса генерации ID
	IDServiceURL string

	// Таймаут на весь HTTP-запрос к upstream
	UpstreamTimeout time.Duration

	// Rate limiter: максимум запросов в секунду на один IP
	RateLimitRPS int

	// Rate limiter: размер всплеска (burst)
	RateLimitBurst int
}

func LoadGateway() GatewayConfig {
	return GatewayConfig{
		Port:            config.GetEnv("GATEWAY_PORT", "9090"),
		IDServiceURL:    config.GetEnv("ID_SERVICE_URL", "localhost:50051"),
		UpstreamTimeout: config.GetEnvDuration("UPSTREAM_TIMEOUT", 5*time.Second),
		RateLimitRPS:    config.GetEnvInt("RATE_LIMIT_RPS", 10),
		RateLimitBurst:  config.GetEnvInt("RATE_LIMIT_BURST", 20),
	}
}
