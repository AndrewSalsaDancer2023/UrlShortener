package utils

import "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

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
