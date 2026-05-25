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

	pb "urlshortener/internal/proto/dbservice"
	"urlshortener/internal/restorerservice/handler"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"urlshortener/utils"

	config "urlshortener/internal/dbstorage/config"
	loaderpool "urlshortener/internal/dbstorage/pool/loader"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

/*
import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"://github.com/pgxpool"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"

	pb "project/gen"
)

type RedirectorServer struct {
	pb.UnimplementedLinkServiceServer
	db     *pgxpool.Pool
	valkey valkey.Client
}

// Конструктор внедрения зависимостей
func NewRedirectorServer(db *pgxpool.Pool, vk valkey.Client) *RedirectorServer {
	return &RedirectorServer{
		db:     db,
		valkey: vk,
	}
}

func fromBase62(s string) int64 {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var res int64
	for i := 0; i < len(s); i++ {
		pos := int64(0)
		for j := 0; j < 62; j++ {
			if charset[j] == s[i] {
				pos = int64(j)
				break
			}
		}
		res = res*62 + pos
	}
	return res
}

func (s *RedirectorServer) GetOriginalURL(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	shortKey := req.ShortKey
	cacheShortKey := "ln:short:" + shortKey

	// 1. Быстрый поиск в кэше Valkey
	if longURL, err := s.valkey.Do(ctx, s.valkey.B().Get().Key(cacheShortKey).Build()).ToString(); err == nil {
		return &pb.GetResponse{LongUrl: longURL}, nil
	}

	// 2. Декодируем Base62 строку обратно в 42-битный числовой ID
	id := fromBase62(shortKey)

	// 3. Ищем в REPLICA Postgres (target_session_attrs=read-only)
	var longURL string
	query := `SELECT long_url FROM short_urls WHERE id = $1;`

	err := s.db.QueryRow(ctx, query, id).Scan(&longURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("ссылка не найдена: %s", shortKey)
		}
		return nil, fmt.Errorf("replica database error: %v", err)
	}

	// 4. Записываем обратно в кэш "на лету" для будущих запросов
	s.valkey.Do(ctx, s.valkey.B().Set().Key(cacheShortKey).Value(longURL).Ex(86400).Build())

	return &pb.GetResponse{LongUrl: longURL}, nil
}

func main() {
	// Подключение к Replica Postgres (target_session_attrs=read-only)
	config, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_READ_URL"))
	config.MaxConns = 60 // Расширенный пул для высокого rps на чтение
	config.MinConns = 15
	dbPool, _ := pgxpool.NewWithConfig(context.Background(), config)

	// Подключение к Valkey
	vkClient, _ := valkey.NewClient(valkey.ClientOption{InitAddress: []string{os.Getenv("VALKEY_ADDR")}})

	lis, _ := net.Listen("tcp", ":50052")
	grpcServer := grpc.NewServer()

	// Внедрение зависимостей
	server := NewRedirectorServer(dbPool, vkClient)
	pb.RegisterLinkServiceServer(grpcServer, server)

	log.Println("Redirector (Reader) gRPC Service started on :50052...")
	grpcServer.Serve(lis)
}
*/

func main() {
	// 1. Конфигурация
	cfg := config.GetURLReaderConfig()

	//Создаем контекст с таймаутом в 5 секунд на базе пустого Background-контекста
	//Функция возвращает сам контекст (ctx) и функцию отмены (cancel)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// ОБЯЗАТЕЛЬНО: всегда вызывайте cancel через defer!
	// Это освобождает ресурсы системы (таймеры ОС), как только работа завершится,
	// даже если она завершилась быстрее, чем за 5 секунд.
	defer cancel()

	// 2. Настройка gRPC-слоя и Перехватчиков (Middleware)
	// Настройка Logger Interceptor (адаптируем стандартный логгер Go под gRPC)
	logFileName := "grpc_server" + cfg.Port + ".log"
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Не удалось открыть файл логов %s: %v", logFileName, err)
	}
	// Обязательно закрываем файл при завершении работы всего приложения
	defer logFile.Close()
	log.SetOutput(logFile)

	// 3. Низкоуровневые зависимости
	pool, err := loaderpool.New(ctx, &cfg)
	if err != nil {
		log.Fatalf("failed to db pool object: %v", err)
	}

	err = pool.TryConnect(ctx)
	if err != nil {
		log.Fatalf("failed connect to db: %v", err)
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

	// 6. Регистрация обработчика (вместо h.NewRouter())
	grpcHandler := handler.New(pool)
	pb.RegisterReadUrlServiceServer(gRPCServer, grpcHandler)

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
