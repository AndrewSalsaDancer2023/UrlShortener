package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
)

const (
	ConfigPath         = "data/balance_config.json"
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

/*var levelStr string
switch lvl {
case logging.LevelDebug:
	levelStr = "DEBUG"
case logging.LevelInfo:
	levelStr = "INFO"
case logging.LevelWarn:
	levelStr = "WARN"
case logging.LevelError:
	levelStr = "ERROR"
default:
	levelStr = "UNKNOWN"
}*/
