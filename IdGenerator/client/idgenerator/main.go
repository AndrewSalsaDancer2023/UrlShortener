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
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	client "urlshortener/internal/idgenerator/client"
	pb "urlshortener/internal/proto/idservice"
)

var (
	addr             = "localhost:50051"
	count            = 10
	lookaheadBatches = 2
	rpcTimeout       = 2 * time.Second
	dialTimeout      = 5 * time.Second
)

func main() {

	logFileName := "grpc_client" + ".log"
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Не удалось открыть файл логов %s: %v", logFileName, err)
		return
	}
	// Обязательно закрываем файл при завершении работы всего приложения
	defer logFile.Close()
	log.SetOutput(logFile)

	// ctx на всё приложение: Ctrl+C/SIGTERM корректно останавливают
	// как цикл получения ID, так и graceful-закрытие пула/соединения.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	// 1. Устанавливаем gRPC-соединение с сервисом.
	//    insecure.NewCredentials() — без TLS, т.к. сервис из этого проекта
	//    поднимается без него; для прода замените на реальные credentials.
	dialCtx, dialCancel := context.WithTimeout(ctx, dialTimeout)
	defer dialCancel()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("failed to create grpc client for %s: %v", addr, err)
		return
	}
	defer conn.Close()

	// Явно дожидаемся установления соединения, чтобы сразу увидеть
	// ошибку конфигурации/недоступности сервиса, а не на первом NextID.
	conn.Connect()
	if !waitForReady(dialCtx, conn) {
		log.Printf("connection to %s did not become ready within %s", addr, dialTimeout)
		return
	}

	// 2. Создаём сгенерированный gRPC-клиент сервиса.
	grpcClient := pb.NewIDServiceClient(conn)

	// 3. Оборачиваем его в client.Pool — с этого момента в фоне уже
	//    запущена предвыборка батчей (см. internal/client/pool.go).
	pool := client.NewPool(ctx, grpcClient, lookaheadBatches, rpcTimeout, &wg)
	defer pool.Close()

	log.Printf("connected to %s, lookahead=%d batches, requesting IDs...", addr, lookaheadBatches)

	// 4. Получаем ID в цикле.
	got := 0
	for count == 0 || got < count {
		id, err := pool.NextID(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("stopped: %v", ctx.Err())
				break
			}
			log.Printf("NextID failed: %v", err)
			break
		}

		log.Printf("id=%d", id)
		got++
		time.Sleep(1 * time.Second)
	}

	log.Printf("done: received %d ids", got)

	pool.Close()

	// 5. Даем время на плавное закрытие всего приложения
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Канал для отслеживания завершения всех горутин (включая пул)
	done := make(chan struct{})
	go func() {
		wg.Wait() // Ждем, пока отработает defer wg.Done() внутри пула
		close(done)
	}()

	select {
	case <-done:
		log.Println("Фоновая горутина пула успешно завершила работу.")
	case <-shutdownCtx.Done():
		log.Println("Внимание: таймаут завершения превышен, принудительный выход.")
	}
}

// waitForReady ждёт, пока conn перейдёт в состояние Ready, либо пока не
// истечёт ctx. Возвращает false при таймауте/отмене.
func waitForReady(ctx context.Context, conn *grpc.ClientConn) bool {
	for {
		state := conn.GetState()
		if state == connectivity.Ready { // ИСПРАВЛЕНО: типизированная проверка
			return true
		}
		if !conn.WaitForStateChange(ctx, state) {
			return false
		}
	}
}
