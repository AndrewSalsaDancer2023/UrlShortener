package grpclient

import (
	"context"
	"fmt"
	"urlshortener/internal/gateway/handler/domain"
	pbc "urlshortener/internal/proto/cacheservice"
	pbdb "urlshortener/internal/proto/dbservice"
	pb "urlshortener/internal/proto/idservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

type ClientConfig struct {
	Scheme        string
	Path          string
	Addrs         []string
	ServiceConfig string
}

type BaseClient struct {
	conn *grpc.ClientConn
	rslv *manual.Resolver
}

func (b *BaseClient) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

type GRPCIDServiceClient struct {
	BaseClient // Встраивание (Embedding). Дает авто-доступ к методу Close()
	client     pb.IDServiceClient
}

type GRPCURLShortenerClient struct {
	BaseClient
	client pbdb.StoreUrlServiceClient
}

type GRPCURLRestorerClient struct {
	BaseClient
	client pbdb.ReadUrlServiceClient
}

type GRPCURLCacheClient struct {
	BaseClient
	client pbc.UrlCacheServiceClient
}

/*
// GRPC-клиент к микросервису генерации ID.

	type GRPCIDServiceClient struct {
		client pb.IDServiceClient
		conn   *grpc.ClientConn
		rslv   *manual.Resolver
	}

	type GRPCURLShortenerClient struct {
		client pbdb.StoreUrlServiceClient
		conn   *grpc.ClientConn
		rslv   *manual.Resolver
	}

	type GRPCURLRestorerClient struct {
		client pbdb.ReadUrlServiceClient
		conn   *grpc.ClientConn
		rslv   *manual.Resolver
	}
*/

func InitResolvers(scheme string, addrs []string) *manual.Resolver {
	rb := manual.NewBuilderWithScheme(scheme)

	addresses := make([]resolver.Address, len(addrs))
	for i, addr := range addrs {
		addresses[i] = resolver.Address{Addr: addr}
	}

	rb.InitialState(resolver.State{
		Addresses: addresses,
	})

	return rb
}

/*
func CreateClientConnection(scheme string, path string, addrs []string, serviceConfig string) (*grpc.ClientConn, *manual.Resolver, error) {
	rslv := InitResolvers(scheme, addrs)
	client, err := grpc.NewClient(
		scheme+":///"+path,       // Имя схемы + виртуальный путь
		grpc.WithResolvers(rslv), // Регистрируем наш резолвер
		grpc.WithDefaultServiceConfig(serviceConfig),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		// Если не удалось создать клиент, возвращаем явную ошибку
		return nil, nil, fmt.Errorf("failed to create grpc connection to id server: %w", err)
	}

	return client, rslv, nil
}
*/

func CreateClientConnection(cfg ClientConfig) (*grpc.ClientConn, *manual.Resolver, error) {
	rslv := InitResolvers(cfg.Scheme, cfg.Addrs)

	target := fmt.Sprintf("%s:///%s", cfg.Scheme, cfg.Path)
	// 	target	"local-cluster:///id-service-endpoints", // Имя схемы + виртуальный путь
	client, err := grpc.NewClient(
		target,
		grpc.WithResolvers(rslv),
		grpc.WithDefaultServiceConfig(cfg.ServiceConfig),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create grpc connection to %s: %w", target, err)
	}

	return client, rslv, nil
}

// 6. МАГИЯ GENERICS: Одна функция для создания ЛЮБОГО gRPC клиента.
// T - это тип вашего кастомного клиента (например, GRPCIDServiceClient)
// ClientAPI - это тип gRPC интерфейса из сгенерированного proto-файла
func NewClient[T any, ClientAPI any](cfg ClientConfig, newServiceFunc func(grpc.ClientConnInterface) ClientAPI, initFunc func(BaseClient, ClientAPI) *T) (*T, error) {
	conn, res, err := CreateClientConnection(cfg)
	if err != nil {
		return nil, err
	}

	base := BaseClient{conn: conn, rslv: res}
	apiClient := newServiceFunc(conn)

	return initFunc(base, apiClient), nil
}

func NewGenIDClient(cfg ClientConfig) (*GRPCIDServiceClient, error) {
	return NewClient(cfg, pb.NewIDServiceClient, func(b BaseClient, api pb.IDServiceClient) *GRPCIDServiceClient {
		return &GRPCIDServiceClient{BaseClient: b, client: api}
	})
}

func (c *GRPCIDServiceClient) Generate(ctx context.Context) (*domain.GenerateResponse, error) {
	resp, err := c.client.GetNextID(ctx, &pb.IDRequest{})
	if err != nil {
		return nil, fmt.Errorf("grpc request failed: %w", err)
	}

	return &domain.GenerateResponse{NumericID: resp.GetId(), ShortCode: resp.GetCode()}, nil
}

func NewURLShortenClient(cfg ClientConfig) (*GRPCURLShortenerClient, error) {
	return NewClient(cfg, pbdb.NewStoreUrlServiceClient, func(b BaseClient, api pbdb.StoreUrlServiceClient) *GRPCURLShortenerClient {
		return &GRPCURLShortenerClient{BaseClient: b, client: api}
	})
}

func (c *GRPCURLShortenerClient) Shorten(ctx context.Context, short_url int64, long_url string) (int64, error) {
	resp, err := c.client.WriteURLPair(ctx, &pbdb.CreateRequest{ShortUrl: short_url, LongUrl: long_url})

	if err != nil {
		return 0, fmt.Errorf("grpc request failed: %w", err)
	}

	return resp.GetShortUrl(), nil
}

func NewURLRestorerClient(cfg ClientConfig) (*GRPCURLRestorerClient, error) {
	return NewClient(cfg, pbdb.NewReadUrlServiceClient, func(b BaseClient, api pbdb.ReadUrlServiceClient) *GRPCURLRestorerClient {
		return &GRPCURLRestorerClient{BaseClient: b, client: api}
	})
}

func (c *GRPCURLRestorerClient) Restore(ctx context.Context, short_url int64) (string, error) {
	resp, err := c.client.GetLongURL(ctx, &pbdb.GetRequest{ShortUrl: short_url})

	if err != nil {
		return "", fmt.Errorf("grpc request failed: %w", err)
	}

	return resp.GetLongUrl(), nil
}

func NewURLCacheClient(cfg ClientConfig) (*GRPCURLCacheClient, error) {
	return NewClient(cfg, pbc.NewUrlCacheServiceClient, func(b BaseClient, api pbc.UrlCacheServiceClient) *GRPCURLCacheClient {
		return &GRPCURLCacheClient{BaseClient: b, client: api}
	})
}

func (c *GRPCURLCacheClient) SaveURLPair(ctx context.Context, short_url int64, long_url string) error /*(int64, error)*/ {
	/*resp*/ _, err := c.client.WriteURLPair(ctx, &pbc.CreateRequest{ShortUrl: short_url, LongUrl: long_url})

	if err != nil {
		return fmt.Errorf("grpc WriteURLPair method for cache engine failed: %w", err)
	}

	//return resp.GetShortUrl(), nil
	return nil
}

func (c *GRPCURLCacheClient) GetShortURL(ctx context.Context, short_url int64, long_url string) (int64, error) {
	resp, err := c.client.GetShortURL(ctx, &pbc.GetShortURLRequest{ShortUrl: short_url, LongUrl: long_url})
	if err != nil {
		return 0, fmt.Errorf("grpc GetShortURL method for cache engine failed: %w", err)
	}

	return resp.GetShortUrl(), nil
}

func (c *GRPCURLCacheClient) GetLongURL(ctx context.Context, short_url int64) (string, error) {
	resp, err := c.client.GetLongURL(ctx, &pbc.GetLongURLRequest{ShortUrl: short_url})
	if err != nil {
		return "", fmt.Errorf("grpc GetLongURL method for cache engine failed: %w", err)
	}

	return resp.GetLongUrl(), nil
}

/*
func NewGenIDClient(scheme string, path string, addrs []string, serviceConfig string) (*GRPCIDServiceClient, error) {
	conn, res, err := CreateClientConnection(scheme, path, addrs, serviceConfig)
	if err != nil {
		return nil, err
	}
	return &GRPCIDServiceClient{
			// baseURL: "",
			client: pb.NewIDServiceClient(conn),
			conn:   conn,
			rslv:   res,
		},
		nil
}

func (c *GRPCIDServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func NewURLShortenClient(scheme string, path string, addrs []string, serviceConfig string) (*GRPCURLShortenerClient, error) {
	conn, res, err := CreateClientConnection(scheme, path, addrs, serviceConfig)
	if err != nil {
		return nil, err
	}

	return &GRPCURLShortenerClient{
			// baseURL: "",
			client: pbdb.NewStoreUrlServiceClient(conn),
			conn:   conn,
			rslv:   res,
		},
		nil
}

func (c *GRPCURLShortenerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func NewURLRestorerClient(scheme string, path string, addrs []string, serviceConfig string) (*GRPCURLRestorerClient, error) {
	conn, res, err := CreateClientConnection(scheme, path, addrs, serviceConfig)
	if err != nil {
		return nil, err
	}

	return &GRPCURLRestorerClient{
			// baseURL: "",
			client: pbdb.NewReadUrlServiceClient(conn),
			conn:   conn,
			rslv:   res,
		},
		nil
}

func (c *GRPCURLRestorerClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
*/
///////////////////////////////////////////////
