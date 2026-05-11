package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"urlshortener/config"
	"urlshortener/internal/base62"

	"urlshortener/internal/generator"

	service "urlshortener/internal/idgenservice"
	"urlshortener/internal/idgenservice/handler"
	pb "urlshortener/internal/proto"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	// 1. Конфигурация
	cfg := config.Load()

	// 2. Низкоуровневые зависимости
	gen, err := generator.New(generator.Config{
		//		Epoch:        cfg.Epoch,
		DatacenterID: cfg.DatacenterID,
		MachineID:    cfg.MachineID,
	})
	if err != nil {
		log.Fatalf("failed to create generator: %v", err)
	}

	encoder := base62.New()

	// 3. Настройка gRPC-слоя и Перехватчиков (Middleware)

	svc := service.New(gen, encoder /*, bus*/)
	// Настройка Logger Interceptor (адаптируем стандартный логгер Go под gRPC)
	loggerOpts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}
	grpcLogger := logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		var levelStr string
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
		}
		log.Printf("[%s] %s %v", levelStr, msg, fields)
	})

	//4. Настройка Recover Interceptor (перехват panic)
	recoveryOpts := []recovery.Option{
		recovery.WithRecoveryHandler(func(p any) (err error) {
			log.Printf("Перехвачена критическая ошибка (panic): %v", p)
			return status.Errorf(codes.Internal, "Внутренняя ошибка сервера")
		}),
	}

	//5. Создаем gRPC сервер и объединяем перехватчики в цепочку (Слева направо: Recover -> Logger -> Router)
	gRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recoveryOpts...),
			logging.UnaryServerInterceptor(grpcLogger, loggerOpts...),
		),
	)

	// 6. Регистрация вашего обработчика (вместо h.NewRouter())
	// Передаем наш svc в структуру, реализующую сгенерированный gRPC-интерфейс
	grpcHandler := handler.New(svc)
	pb.RegisterIDServiceServer(gRPCServer, grpcHandler)

	//7. gRPC работает поверх чистого TCP соединения, открываем порт
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("failed to listen port %s: %v", cfg.Port, err)
	}

	// 8. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("gRPC server listening on :%s", cfg.Port)
		if err := gRPCServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down gRPC server...")

	// У gRPC есть встроенный метод GracefulStop().
	// Он блокирует поток, ждет завершения всех активных RPC-запросов и закрывает сервер.
	// Привязывать контекст с таймаутом вручную здесь не требуется.
	gRPCServer.GracefulStop()

	log.Println("gRPC server stopped")
}

//grpcurl -plaintext -import-path ./internal/proto -proto idservice.proto -d '{}' localhost:50051  generator.IDService.GetNextID
