package handler

import (
	"context"
	"fmt"
	"time"
	"urlshortener/internal/dbstorage/pool"
	pb "urlshortener/internal/proto/dbservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedReadUrlServiceServer
	// Внедряем сервис бизнес-логики как зависимость
	pool        pool.DBGetURLPool
	readTiemout time.Duration
}

func New(restpool pool.DBGetURLPool, timeOut time.Duration) *GRPCHandler {
	return &GRPCHandler{
		pool:        restpool,
		readTiemout: timeOut,
	}
}

func (h *GRPCHandler) GetLongURL(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, h.readTiemout)
	defer cancel() // Обязательно освобождаем ресурсы в конце
	long_url, err := h.pool.Load(timeoutCtx, req.GetShortUrl())

	if err != nil {
		return nil, fmt.Errorf("failed to store url pair %w", err)
	}

	return &pb.GetResponse{LongUrl: long_url}, nil
}
