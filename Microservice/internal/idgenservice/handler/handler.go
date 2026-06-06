package handler

import (
	"context"
	"fmt"
	service "urlshortener/internal/idgenservice"
	pb "urlshortener/internal/proto/idservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedIDServiceServer

	// Внедряем сервис бизнес-логики как зависимость
	gsv *service.IDService
}

// New — конструктор для создания вашего gRPC-обработчика
func New(svc *service.IDService) *GRPCHandler {
	return &GRPCHandler{
		gsv: svc,
	}
}

func (h *GRPCHandler) GetNextID(ctx context.Context, req *pb.IDRequest) (*pb.IDResponse, error) {
	id, err := h.gsv.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate id: %w", err)
	}

	result := &pb.IDResponse{Id: id.NumericID, Code: id.ShortCode}
	return result, nil
}
