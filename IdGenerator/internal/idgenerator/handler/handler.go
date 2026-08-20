package handler

import (
	"context"
	"log"
	"urlshortener/internal/idgenerator"
	pb "urlshortener/internal/proto/idservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedIDServiceServer

	// Внедряем сервис бизнес-логики как зависимость
	buffer idgenerator.IDBuffer
}

// New — конструктор для создания вашего gRPC-обработчика
func NewHandler(buf idgenerator.IDBuffer) *GRPCHandler {
	return &GRPCHandler{
		buffer: buf,
	}
}

func (h *GRPCHandler) GetIDBatch(ctx context.Context, req *pb.GetBatchRequest) (*pb.GetBatchResponse, error) {
	log.Println("GetIDBatch called!")
	batch, err := h.buffer.TakeBatch(ctx) // забрать готовый батч из общего буфера
	if err != nil {
		return nil, err
	}
	return &pb.GetBatchResponse{Ids: batch}, nil
}
