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
	pb.UnimplementedStoreUrlServiceServer
	// Внедряем сервис бизнес-логики как зависимость
	pool         pool.DBSaveURLPool
	timeEngine   pool.DBTime
	writeTiemout time.Duration
}

func New(savepool pool.DBSaveURLPool, timeEngine pool.DBTime, timeOut time.Duration) *GRPCHandler {
	return &GRPCHandler{
		pool:         savepool,
		timeEngine:   timeEngine,
		writeTiemout: timeOut,
	}
}

func (h *GRPCHandler) WriteURLPair(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, h.writeTiemout)
	defer cancel() // Обязательно освобождаем ресурсы в конце
	url_id, err := h.pool.Save(timeoutCtx, req.GetShortUrl(), req.GetLongUrl(), h.timeEngine)

	if err != nil {
		return nil, fmt.Errorf("failed to store url pair %w", err)
	}

	return &pb.CreateResponse{ShortUrl: url_id}, nil
}
