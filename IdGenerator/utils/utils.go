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

// Пытается получить номер пода
func ExtractPodNumber(hostName string) (int64, error) {
	// Находим последний дефис в имени хоста
	lastDash := strings.LastIndex(hostName, "-")
	if lastDash == -1 || lastDash == len(hostName)-1 {
		// Если дефиса нет, значит код запущен локально (например, на Windows/Mac)
		// Возвращаем ошибку или дефолтный ID для локальной разработки
		return 0, fmt.Errorf("hostname %s does not contain a valid pod index", hostName)
	}

	// Вырезаем подстроку после дефиса (например, из "idgen-1" получим "1")
	indexStr := hostName[lastDash+1:]

	nodeID, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse node ID from substring %s: %w", indexStr, err)
	}

	return nodeID, nil
}

// считывает имя хоста пода и извлекает его порядковый номер
func ExtractNodeID() (int64, error) {
	// В Kubernetes StatefulSet hostname всегда равен имени пода (например, "idgen-0")
	hostName, err := os.Hostname()
	if err != nil {
		return 0, fmt.Errorf("failed to get hostname: %w", err)
	}

	return ExtractPodNumber(hostName)
}

func ExtractMachineAndDataCenterID(nodeId int64) (datacenterID int64, machineID int64) {
	datacenterID = nodeId >> 1 // сдвиг на 1 бит вправо даст ID датацентра
	machineID = nodeId & 1     // остаток даст ID машины
	return datacenterID, machineID
}

// Функция-помощник для установки дефолтного значения
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key) // Ищет строго в том регистре, что в env
	if len(value) == 0 {
		return defaultValue
	}
	return value
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
