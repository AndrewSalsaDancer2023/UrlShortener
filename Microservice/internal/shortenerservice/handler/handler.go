package handler

import (
	"context"
	"fmt"
	"urlshortener/internal/dbstorage/pool"
	pb "urlshortener/internal/proto/dbservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedStoreUrlServiceServer
	// Внедряем сервис бизнес-логики как зависимость
	pool  pool.DBSaveURLPool
	timer pool.DBTime
}

func New(savepool pool.DBSaveURLPool, timer pool.DBTime) *GRPCHandler {
	return &GRPCHandler{
		pool:  savepool,
		timer: timer,
	}
}

func (h *GRPCHandler) WriteURLPair(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	url_id, err := h.pool.Save(ctx, req.GetShortUrl(), req.GetLongUrl(), h.timer)

	if err != nil {
		return nil, fmt.Errorf("failed to store url pair %w", err)
	}

	return &pb.CreateResponse{ShortUrl: url_id}, nil
}
