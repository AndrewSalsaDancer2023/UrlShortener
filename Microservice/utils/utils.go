package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	// "urlshortener/internal/dbstorage/pool"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

type PingClient interface {
	Ping(context.Context) error
}

const (
	ConfigPath         = "../../data/balance_config.json"
	GateWayLogFileName = "gateway.log"
)

func CreateDebugLevelString(lvl logging.Level) string {
	switch lvl {
	case logging.LevelDebug:
		return "DEBUG"
	case logging.LevelInfo:
		return "INFO"
	case logging.LevelWarn:
		return "WARN"
	case logging.LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func ReadJSONFile(filePath string) (string, error) {
	//1. Читаем файл с диска
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 2. Превращаем байты в строку и убираем лишние пробелы/переносы строк
	line := strings.TrimSpace(string(data))

	// 3. Проверяем, является ли строка валидным JSON
	if json.Valid([]byte(line)) == false {
		return "", fmt.Errorf("invalid JSON file: %s", filePath)
	}
	return line, nil
}

func GetURLHash(longURL string) string {
	hash := sha256.Sum256([]byte(longURL))
	hashString := hex.EncodeToString(hash[:])
	return hashString
}

func CeateShortURLKey(shortURL int64) string {
	// shortKey := "url:{" + strconv.FormatInt(shortURL, 10) + "}"
	shortKey := strconv.FormatInt(shortURL, 10)
	return shortKey
}

func CreateHashedURLKey( /*shortURL int64,*/ longURL string) string {
	// longKey := "urlhashed:{" + strconv.FormatInt(shortURL, 10) + "}:" + GetURLHash(longURL)
	longKey := GetURLHash(longURL)
	return longKey
}

func CreateLogFile(logFileName string) *os.File {
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Не удалось открыть файл логов %s: %v", logFileName, err)
	}
	return logFile
}

func StartRedisHealthCheck(ctx context.Context, healthServer *health.Server, redisClient PingClient) {
	// Создаем таймер, например, проверять каждые 5 секунд
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Если приложение останавливается, выходим из горутины
			return
		case <-ticker.C:
			// Создаем короткий таймаут конкретно на операцию пинга (например, 2 секунды)
			pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

			err := redisClient.Ping(pingCtx)
			cancel() // Освобождаем ресурсы контекста сразу после пинга

			if err != nil {
				// Логируем ошибку, но НЕ дропаем приложение через Fatalf!
				log.Printf("Health Check: Redis is down: %v", err)

				// Переключаем gRPC статус в NOT_SERVING
				healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
			} else {
				// Если всё хорошо — подтверждаем статус SERVING
				healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
			}
		}
	}
}
