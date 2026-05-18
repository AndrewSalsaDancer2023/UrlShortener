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
	"urlshortener/internal/shortenerservice/handler"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"urlshortener/utils"

	config "urlshortener/internal/dbstorage/config"
	saverpool "urlshortener/internal/dbstorage/pool/saver"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

/*
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	pb "project/gen" // замените на ваш реальный путь к gen
)

const (
	CustomEpoch  = 1778112000 // 2026-05-05 00:00:00 UTC
	SequenceMask = 2047       // 11 бит
)

type ShortenerServer struct {
	pb.UnimplementedLinkServiceServer
	db            *pgxpool.Pool
	valkey        valkey.Client
	mu            sync.Mutex
	lastTimestamp int64
	serverID      int64
	sequence      int64
}

// Конструктор внедрения зависимостей
func NewShortenerServer(db *pgxpool.Pool, vk valkey.Client, serverID int64) *ShortenerServer {
	return &ShortenerServer{
		db:       db,
		valkey:   vk,
		serverID: serverID,
	}
}


func (s *ShortenerServer) CreateShortURL(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	longURL := req.LongUrl
	hashSum := sha256.Sum256([]byte(longURL))
	urlHash := hex.EncodeToString(hashSum[:])

	// 1. Проверяем кэш по хешу длинного URL
	cacheHashKey := "ln:hash:" + urlHash
	if val, err := s.valkey.Do(ctx, s.valkey.B().Get().Key(cacheHashKey).Build()).ToString(); err == nil {
		return &pb.CreateResponse{ShortKey: val}, nil
	}

	// 2. Генерация 42-битного Snowflake ID
	s.mu.Lock()
	now := time.Now().Unix() - CustomEpoch
	if now < s.lastTimestamp {
		s.mu.Unlock()
		return nil, errors.New("критический сбой: время на сервере ушло назад")
	}

	if now == s.lastTimestamp {
		s.sequence = (s.sequence + 1) & SequenceMask
		if s.sequence == 0 {
			s.mu.Unlock()
			// Пассивное ожидание начала новой секунды
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Until(time.Unix(now+CustomEpoch+1, 0))):
				return s.CreateShortURL(ctx, req)
			}
		}
	} else {
		s.sequence = 0
	}
	s.lastTimestamp = now
	id := (now << 14) | (s.serverID << 11) | s.sequence
	s.mu.Unlock()

	// 3. Запись в Master Postgres (UPSERT)
	var finalID int64
	query := `INSERT INTO short_urls (id, long_url) VALUES ($1, $2)
              ON CONFLICT (long_url) DO UPDATE SET long_url = EXCLUDED.long_url
              RETURNING id`

	if err := s.db.QueryRow(ctx, query, id, longURL).Scan(&finalID); err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	shortKey := toBase62(finalID)

	// 4. Наполнение кэша Valkey (Два ключа атомарно через Pipeline)
	cacheShortKey := "ln:short:" + shortKey

	// Создаем пайплайн для одновременной записи
	s.valkey.DoMulti(ctx,
		s.valkey.B().Set().Key(cacheShortKey).Value(longURL).Ex(86400).Build(),      // Для Редиректора
		s.valkey.B().Set().Key(cacheHashKey).Value(shortKey).Nx().Ex(86400).Build(), // Для Сокращателя
	)

	return &pb.CreateResponse{ShortKey: shortKey}, nil
}

func main() {
	// Подключение к Master Postgres (target_session_attrs=read-write)
	config, _ := pgxpool.ParseConfig(os.Getenv("DATABASE_WRITE_URL"))
	config.MaxConns = 20
	dbPool, _ := pgxpool.NewWithConfig(context.Background(), config)

	// Подключение к Valkey
	vkClient, _ := valkey.NewClient(valkey.ClientOption{InitAddress: []string{os.Getenv("VALKEY_ADDR")}})

	lis, _ := net.Listen("tcp", ":50051")
	grpcServer := grpc.NewServer()

	// Внедрение зависимостей
	server := NewShortenerServer(dbPool, vkClient, 1)
	pb.RegisterLinkServiceServer(grpcServer, server)

	log.Println("Shortener (Writer) gRPC Service started on :50051...")
	grpcServer.Serve(lis)
}
*/

func main() {
	// 1. Конфигурация
	cfg := config.GetURLSaverConfig()

	// 1. Создаем контекст с таймаутом в 5 секунд на базе пустого Background-контекста
	// Функция возвращает сам контекст (ctx) и функцию отмены (cancel)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// 2. ОБЯЗАТЕЛЬНО: всегда вызывайте cancel через defer!
	// Это освобождает ресурсы системы (таймеры ОС), как только работа завершится,
	// даже если она завершилась быстрее, чем за 5 секунд.
	defer cancel()

	// 2. Низкоуровневые зависимости
	pool, err := saverpool.New(ctx, &cfg)
	if err != nil {
		log.Fatalf("failed to db pool object: %v", err)
	}

	// 3. Настройка gRPC-слоя и Перехватчиков (Middleware)

	//	svc := service.New(pool)
	// Настройка Logger Interceptor (адаптируем стандартный логгер Go под gRPC)
	logFileName := "grpc_server" + cfg.Port + ".log"
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Не удалось открыть файл логов %s: %v", logFileName, err)
	}
	// Обязательно закрываем файл при завершении работы всего приложения
	defer logFile.Close()
	log.SetOutput(logFile)

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
	//grpcHandler := handler.New(svc)
	grpcHandler := handler.New(pool)
	pb.RegisterStoreUrlServiceServer(gRPCServer, grpcHandler)

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
