package handler

import (
	"context"
	"fmt"
	"urlshortener/internal/dbstorage/pool"
	pb "urlshortener/internal/proto/dbservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedReadUrlServiceServer
	// Внедряем сервис бизнес-логики как зависимость
	pool pool.DBGetURLPool
}

func New(restpool pool.DBGetURLPool) *GRPCHandler {
	return &GRPCHandler{
		pool: restpool,
	}
}

func (h *GRPCHandler) GetLongURL(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	long_url, err := h.pool.Load(ctx, req.GetShortUrl())

	if err != nil {
		return nil, fmt.Errorf("failed to store url pair %w", err)
	}

	return &pb.GetResponse{LongUrl: long_url}, nil
}
