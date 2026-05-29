package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"urlshortener/internal/cacheservice/handler"
	pb "urlshortener/internal/proto/cacheservice"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"urlshortener/utils"

	cache "urlshortener/internal/cacheservice"
	config "urlshortener/internal/cacheservice/config"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// 1. Конфигурация
	cfg := config.GetURLCacheConfig()

	// 2. Настройка Logger Interceptor (адаптируем стандартный логгер Go под gRPC)
	//logFileName := "grpc_server" + cfg.Port + ".log"
	/*	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	 */
	logFile := utils.CreateLogFile("grpc_server" + cfg.Port + ".log")
	// Обязательно закрываем файл при завершении работы всего приложения
	defer logFile.Close()
	log.SetOutput(logFile)

	// 3. Низкоуровневые зависимости
	srv := cache.New(&cfg)
	// Создаем контекст с таймаутом в 5 секунд на базе пустого Background-контекста
	/*	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// ОБЯЗАТЕЛЬНО: всегда вызывайте cancel через defer!
		defer cancel()
		err = srv.Ping(ctx)*/
	err := srv.TryConnectToCache()
	if err != nil {
		log.Fatalf("Unable connect to cache service %v", err)
	}

	loggerOpts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}
	grpcLogger := logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		log.Printf("[%s] %s %v", utils.CreateDebugLevelString(lvl), msg, fields)
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
	grpcHandler := handler.New(srv, &cfg)
	pb.RegisterUrlCacheServiceServer(gRPCServer, grpcHandler)

	// 7. Создаем health сервер и регистрируем наш gRPCServer
	//в качестве наблюдаемого
	healthServer := health.NewServer()
	// Устанавливаем статус SERVING
	healthServer.SetServingStatus(
		"",
		healthgrpc.HealthCheckResponse_SERVING,
	)
	healthgrpc.RegisterHealthServer(
		gRPCServer,
		healthServer,
	)

	//8. gRPC работает поверх чистого TCP соединения, открываем порт
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("failed to listen port %s: %v", cfg.Port, err)
	}

	// 9. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Создаем отменяемый контекст для фоновых задач приложения (включая healthcheck)
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Запускаем ваш healthcheck, передавая ему appCtx
	go utils.StartRedisHealthCheck(appCtx, healthServer, srv)

	go func() {
		log.Printf("gRPC server listening on :%s", cfg.Port)
		if err := gRPCServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down gRPC server...")

	// 1. Срочно говорим всем балансировщикам: "Мы выключаемся, не шлите сюда людей!"
	healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	// 2. Останавливаем фоновую горутину пинга Redis
	appCancel()

	// (Опционально) Даем балансировщику 2-3 секунды, чтобы он успел обновить свои таблицы
	// и перенаправить новые запросы на другие поды, пока мы еще физически не закрыли порт.
	time.Sleep(3 * time.Second)

	// У gRPC есть встроенный метод GracefulStop().
	// Он блокирует поток, ждет завершения всех активных RPC-запросов и закрывает сервер.
	// Привязывать контекст с таймаутом вручную здесь не требуется.
	gRPCServer.GracefulStop()

	log.Println("gRPC server stopped")
}

//for debugging purpose: sudo lsof -i :5057
//sudo kill 1234
/*
grpcurl -plaintext -import-path ./internal/proto -proto cacheservice.proto -d '{"short_url": 12345, "long_url": "https://google.com"}' localhost:50057 cacheservice.UrlCacheService.WriteURLPair

grpcurl -plaintext -import-path ./internal/proto -proto cacheservice.proto -d '{"short_url": 12345}' localhost:50057 cacheservice.UrlCacheService.GetLongURL

grpcurl -plaintext -import-path ./internal/proto -proto cacheservice.proto -d '{"long_url": "https://google.com"}' localhost:50057 cacheservice.UrlCacheService.GetShortURL
*/
