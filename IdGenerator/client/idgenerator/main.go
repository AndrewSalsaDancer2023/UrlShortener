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
	"errors"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	client "urlshortener/internal/idgenerator/client"
	pb "urlshortener/internal/proto/idservice"
	"urlshortener/utils"
)

var (
	addr             = "localhost:50051"
	count            = 10
	lookaheadBatches = 2
	rpcTimeout       = 2 * time.Second
	dialTimeout      = 5 * time.Second
)

func main() {

	// 1. Инициализируем логирование
	logFile := utils.CreateLogFile("grpc_client.log")
	defer logFile.Close()
	log.SetOutput(logFile)

	// 2. Настраиваем системный контекст для Ctrl+C / SIGTERM
	// ctx на всё приложение: Ctrl+C/SIGTERM корректно останавливают
	// как цикл получения ID, так и graceful-закрытие пула/соединения.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 3. Подключаемся к gRPC серверу
	conn, err := setupClientgRPCConnection(ctx, addr, dialTimeout)
	if err != nil {
		log.Printf("Unable to establish gRPC connection: %v", err)
		return
	}
	defer conn.Close()

	// 4. Инициализируем клиент и пул предвыборки ID
	var wg sync.WaitGroup
	grpcClient := pb.NewIDServiceClient(conn)
	pool := client.NewPool(ctx, grpcClient, lookaheadBatches, rpcTimeout, &wg)
	defer pool.Close()

	log.Printf("Connected to %s, prefetching=%d batches. ID request...", addr, lookaheadBatches)

	// 5. Запускаем основной цикл получения ID
	runIDConsumerLoop(ctx, pool, count)

	// 6. Выполняем Graceful Shutdown для фоновых горутин пула
	waitForShutdown(&wg, dialTimeout)
}

// setupClientgRPCConnection создает gRPC-соединение и блокирует поток до тех пор, пока оно не станет Ready.
func setupClientgRPCConnection(ctx context.Context, address string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialCtx, dialCancel := context.WithTimeout(ctx, timeout)
	defer dialCancel()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	conn.Connect()
	if !waitForConnectionReady(dialCtx, conn) {
		conn.Close()
		return nil, errors.New("Connection did not reach the READY status within the allotted time")
	}

	return conn, nil
}

// runIDConsumerLoop запрашивает пачки ID из пула согласно лимиту count.
func runIDConsumerLoop(ctx context.Context, pool *client.Pool, maxCount int) {
	got := 0
	for maxCount == 0 || got < maxCount {
		id, err := pool.NextID(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("The loop execution was stopped by the context signal: %v", ctx.Err())
				break
			}
			log.Printf("NextID method error: %v", err)
			break
		}

		log.Printf("Successfully received id=%d", id)
		got++
		time.Sleep(1 * time.Second)
	}
	log.Printf("Cycle completed: %d identifiers received in total", got)
}

// waitForShutdown ожидает завершения всех фоновых задач в WaitGroup с таймаутом.
func waitForShutdown(wg *sync.WaitGroup, timeout time.Duration) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Gorutines finished")
	case <-shutdownCtx.Done():
		log.Println("Waiting timeout expired. Force finish")
	}
}

// waitForConnectionReady ждёт, пока conn перейдёт в состояние Ready, либо пока не
// истечёт ctx. Возвращает false при таймауте/отмене.
func waitForConnectionReady(ctx context.Context, conn *grpc.ClientConn) bool {
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return true
		}
		if !conn.WaitForStateChange(ctx, state) {
			return false
		}
	}
}
