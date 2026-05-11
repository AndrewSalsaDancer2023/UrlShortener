package grpclient

import (
	"context"
	"fmt"
	"time"
	"urlshortener/internal/gateway/handler/domain"
	pb "urlshortener/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// IDServiceClient — GRPC-клиент к микросервису генерации ID.
type GRPCIDServiceClient struct {
	baseURL string
	client  pb.IDServiceClient
	conn    *grpc.ClientConn
}

func New(baseURL string, timeout time.Duration) (*GRPCIDServiceClient, error) {
	conn, err := grpc.NewClient(baseURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		// Если не удалось создать клиент, возвращаем явную ошибку
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	return &GRPCIDServiceClient{
		baseURL: baseURL,
		client:  pb.NewIDServiceClient(conn),
		conn:    conn,
	}, nil
}

func (c *GRPCIDServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *GRPCIDServiceClient) Generate(ctx context.Context) (*domain.GenerateResponse, error) {
	// conn, err := grpc.NewClient(c.baseURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create grpc client: %w", err)
	// }

	// ВНИМАНИЕ: defer conn.Close() здесь закроет соединение ДО отправки запроса!
	// Если вам нужно закрыть его после выполнения метода, оставьте.
	// Но правильнее держать conn открытым на уровне структуры (см. ниже).
	// defer conn.Close()

	// 3. Исправляем имя переменной (grpcClient вместо c) и присваиваем интерфейс
	// grpcClient := pb.NewIDServiceClient(conn)

	// 4. Теперь делаем сам запрос к другому микросервису
	resp, err := c.client.GetNextID(ctx, &pb.IDRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpc request failed: %w", err)
	}

	return &domain.GenerateResponse{NumericID: resp.GetId(), ShortCode: resp.GetCode()}, nil
}
