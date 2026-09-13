// Command client — пример использования client.Pool: подключается к
// сервису генерации ID по gRPC и печатает полученные ID, используя
// клиентскую предвыборку (см. internal/client).
//
// Запуск:
//
//	go run ./cmd/client -addr localhost:50051 -count 1000
//	go run ./cmd/client -addr localhost:50051 -count 0   # бесконечно, до Ctrl+C
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	// "urlshortener/utils"
)

func main() {
	// 1. Инициализируем логирование
	// logFile := utils.CreateLogFile("grpc_client.log")
	// defer logFile.Close()
	// log.SetOutput(logFile)
	log.SetOutput(os.Stdout)

	// 2. Настраиваем системный контекст для Ctrl+C / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := os.Getenv("IDGEN_SERVER_ADDRESS")
	if addr == "" {
		addr = "localhost:50051"
	} else {
		addr = "dns:///" + addr
	}
	// 3. Собираем конфигурацию
	cfg := Config{
		// Addr:             "localhost:50051",
		// Addr:             "dns:///idgen-headless:50051",
		Addr:             addr,
		Count:            10,
		LookaheadBatches: 2,
		RPCTimeout:       2 * time.Second,
		DialTimeout:      5 * time.Second,
	}

	// 4. Запускаем приложение
	app := NewIDConsumerApp(cfg)
	if err := app.Run(ctx); err != nil {
		log.Printf("Application critical error: %v", err)
	}
}
