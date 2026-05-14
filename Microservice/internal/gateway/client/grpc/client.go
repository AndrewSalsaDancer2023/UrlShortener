package grpclient

import (
	"context"
	"fmt"
	"time"
	"urlshortener/internal/gateway/handler/domain"
	pb "urlshortener/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

// IDServiceClient — GRPC-клиент к микросервису генерации ID.
type GRPCIDServiceClient struct {
	baseURL string
	client  pb.IDServiceClient
	conn    *grpc.ClientConn
}

func CreateResolver(scheme string) *manual.Resolver {
	return manual.NewBuilderWithScheme(scheme)
}

func CreateClientConnection(rslv *manual.Resolver, name string) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		// rslv.Scheme()+":///id-service", // Формат: схема:///имя
		rslv.Scheme()+":"+name, // Формат: схема:///имя
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Важно: включаем Round Robin через Service Config
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
	)
}

func UpdateReolverState(rslv *manual.Resolver, serverAddresses []string) {
	var state resolver.State
	for _, addr := range serverAddresses {
		state.Addresses = append(state.Addresses, resolver.Address{Addr: addr})
	}
	rslv.UpdateState(state)
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

func NewClient(serverAddresses string, serviceConfig string) (*GRPCIDServiceClient, error) {
	conn, err := grpc.NewClient(
		serverAddresses,
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		grpc.WithDefaultServiceConfig(serviceConfig),
	)

	if err != nil {
		// Если не удалось создать клиент, возвращаем явную ошибку
		return nil, fmt.Errorf("failed to create grpc connection to id server: %w", err)
	}

	return &GRPCIDServiceClient{
			baseURL: "",
			client:  pb.NewIDServiceClient(conn),
			conn:    conn,
		},
		nil
}

func (c *GRPCIDServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *GRPCIDServiceClient) Generate(ctx context.Context) (*domain.GenerateResponse, error) {
	resp, err := c.client.GetNextID(ctx, &pb.IDRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpc request failed: %w", err)
	}

	return &domain.GenerateResponse{NumericID: resp.GetId(), ShortCode: resp.GetCode()}, nil
}
