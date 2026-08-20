package handler

import (
	"context"
	"errors"
	"log"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"urlshortener/config"
	"urlshortener/internal/idgenerator"
	pb "urlshortener/internal/proto/idservice"
	"urlshortener/utils"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type App struct {
	grpcServer *grpc.Server
	producer   *idgenerator.Producer // Предполагается наличие метода Run(ctx)
	wg         sync.WaitGroup
	port       string
}

func NewApp(cfg config.Config, buf idgenerator.IDBuffer, gen idgenerator.BatchGenerator, batchsize int) *App {
	// 1. Инициализируем продюсера
	producer := idgenerator.NewProducer(gen, buf, batchsize)

	// 2. Опции логирования и восстановления
	loggerOpts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}
	grpcLogger := logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		log.Printf("[%s] %s %v", utils.CreateDebugLevelString(lvl), msg, fields)
	})

	recoveryOpts := []recovery.Option{
		recovery.WithRecoveryHandler(func(p any) (err error) {
			stackTrace := debug.Stack()
			log.Printf("Captured critical error (panic):: %v\n Call stack:\n%s", p, string(stackTrace))
			return status.Errorf(codes.Internal, "Internal server error")
		}),
	}

	// 3. Сборка gRPC сервера
	gRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recoveryOpts...),
			utils.EnforceDeadlineInterceptor(),
			logging.UnaryServerInterceptor(grpcLogger, loggerOpts...),
		),
	)

	// 4. Регистрация обработчиков
	grpcHandler := NewHandler(buf)
	pb.RegisterIDServiceServer(gRPCServer, grpcHandler)

	return &App{
		grpcServer: gRPCServer,
		producer:   producer,
		port:       cfg.Port,
	}
}

// Run запускает все асинхронные компоненты приложения
func (a *App) Run(ctx context.Context, lis net.Listener) error {
	// Создаем буферизированный канал на 2 элемента
	errChan := make(chan error, 2)
	// Запускаем продюсера
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		errChan <- a.producer.Run(ctx)
	}()

	// Запускаем gRPC сервер
	//errChan := make(chan error, 1)
	go func() {
		log.Printf("gRPC server listening on %s", lis.Addr().String())
		if err := a.grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errChan <- err
		}
		// close(errChan)
	}()

	// Ожидаем сигнала отмены контекста (Ctrl+C / SIGTERM) или ошибки сервера
	select {
	case <-ctx.Done():
		log.Printf("selected case case <-ctx.Done():, stopping App")
		return a.Stop()
	case err := <-errChan:
		log.Printf("selected case err := <-errChan:")
		return err
	}
}

// Stop выполняет чистый Graceful Shutdown всех систем
func (a *App) Stop() error {
	log.Println("Inside App Stop() shutting down gRPC server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcDone := make(chan struct{})
	go func() {
		a.grpcServer.GracefulStop()
		close(grpcDone)
	}()

	producerDone := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(producerDone)
	}()

	select {
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded! Forcing stop...")
		a.grpcServer.Stop()
		return errors.New("shutdown timeout exceeded")
	case <-grpcDone:
		select {
		case <-producerDone:
			log.Println("All systems cleanly exited")
		case <-shutdownCtx.Done():
			log.Println("Warning: Producer shutdown timed out")
			return errors.New("shutdown timeout exceeded: producer hung")
		}
	}

	log.Println("gRPC server stopped")
	return nil
}
