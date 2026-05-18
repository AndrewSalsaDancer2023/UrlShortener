package main

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
