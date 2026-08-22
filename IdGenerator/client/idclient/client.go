package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	client "urlshortener/internal/idgenerator/client"
	pb "urlshortener/internal/proto/idservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// Конфигурация вынесена в отдельную структуру для гибкости
type Config struct {
	Addr             string
	Count            int
	LookaheadBatches int
	RPCTimeout       time.Duration
	DialTimeout      time.Duration
}

// IDConsumerApp объединяет зависимости и состояние клиентского приложения
type IDConsumerApp struct {
	cfg  Config
	wg   sync.WaitGroup
	conn *grpc.ClientConn
	pool *client.Pool // Предполагается, что структура клиентского пула из вашего пакета
}

func NewIDConsumerApp(cfg Config) *IDConsumerApp {
	return &IDConsumerApp{
		cfg: cfg,
	}
}

// Run — главная точка входа, управляющая жизненным циклом приложения
func (a *IDConsumerApp) Run(ctx context.Context) error {
	// 1. Подключаемся к gRPC серверу
	if err := a.setupClientgRPCConnection(ctx); err != nil {
		return fmt.Errorf("unable to establish gRPC connection: %w", err)
	}
	defer func() {
		if a.conn != nil {
			a.conn.Close()
		}
	}()

	// 2. Инициализируем пул предвыборки ID
	grpcClient := pb.NewIDServiceClient(a.conn)
	a.pool = client.NewPool(ctx, grpcClient, a.cfg.LookaheadBatches, a.cfg.RPCTimeout, &a.wg)

	log.Printf("Connected to %s, prefetching=%d batches. ID request...", a.cfg.Addr, a.cfg.LookaheadBatches)

	// 3. Запускаем основной цикл получения ID
	a.runIDConsumerLoop(ctx)
	// и закрываем пул после завершения цикла, тем самым завершая горутину продюсера
	a.pool.Close()
	// 4. Выполняем Graceful Shutdown для фоновых горутин пула
	a.waitForShutdown()
	return nil
}

// setupClientgRPCConnection создает gRPC-соединение и блокирует поток до тех пор, пока оно не станет Ready
func (a *IDConsumerApp) setupClientgRPCConnection(ctx context.Context) error {
	dialCtx, dialCancel := context.WithTimeout(ctx, a.cfg.DialTimeout)
	defer dialCancel()

	conn, err := grpc.NewClient(a.cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	conn.Connect()
	if !a.waitForConnectionReady(dialCtx, conn) {
		conn.Close()
		return errors.New("connection did not reach the READY status within the allotted time")
	}

	a.conn = conn
	return nil
}

// запрашивает пачки ID из пула согласно лимиту count
func (a *IDConsumerApp) runIDConsumerLoop(ctx context.Context) {
	got := 0
	for a.cfg.Count == 0 || got < a.cfg.Count {
		id, err := a.pool.NextID(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("The loop was stopped by the context signal: %v", ctx.Err())
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

// waitForShutdown ожидает завершения всех фоновых задач в WaitGroup с таймаутом
func (a *IDConsumerApp) waitForShutdown() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.DialTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Goroutines finished")
	case <-shutdownCtx.Done():
		log.Println("Waiting timeout expired. Force finish")
	}
}

// waitForConnectionReady ждёт, пока conn перейдёт в состояние Ready, либо пока не истечёт ctx
func (a *IDConsumerApp) waitForConnectionReady(ctx context.Context, conn *grpc.ClientConn) bool {
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
