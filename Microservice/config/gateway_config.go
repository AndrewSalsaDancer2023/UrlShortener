package config

import (
	"os"
	"strconv"
	"time"
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
		Port:            getEnv("GATEWAY_PORT", "9090"),
		IDServiceURL:    getEnv("ID_SERVICE_URL", "localhost:50051"),
		UpstreamTimeout: getEnvDuration("UPSTREAM_TIMEOUT", 5*time.Second),
		RateLimitRPS:    getEnvInt("RATE_LIMIT_RPS", 10),
		RateLimitBurst:  getEnvInt("RATE_LIMIT_BURST", 20),
	}
}

/*
	func getEnv(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
*/
func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
