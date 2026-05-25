package handler

import (
	"context"
	srv "urlshortener/internal/cacheservice"
	pb "urlshortener/internal/proto/cacheservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedUrlCacheServiceServer
	service *srv.UrlCacheService
}

func New(srv *srv.UrlCacheService) *GRPCHandler {
	return &GRPCHandler{
		service: srv,
	}
}

func (h *GRPCHandler) WriteURLPair(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	id, err := h.service.WriteURLPair(ctx, req.GetShortUrl(), req.GetLongUrl())
	if err != nil {
		return nil, err
	}

	return &pb.CreateResponse{ShortUrl: id}, nil
}

func (h *GRPCHandler) GetLongURL(ctx context.Context, req *pb.GetLongURLRequest) (*pb.GetLongURLResponse, error) {
	long_url, err := h.service.GetLongURL(ctx, req.GetShortUrl())
	if err != nil {
		return nil, err
	}
	return &pb.GetLongURLResponse{LongUrl: long_url}, nil
}

func (h *GRPCHandler) GetShortURL(ctx context.Context, req *pb.GetShortURLRequest) (*pb.GetShortURLResponse, error) {
	short_url, err := h.service.GetShortURL(ctx, req.ShortUrl, req.LongUrl)
	if err != nil {
		return nil, err
	}
	return &pb.GetShortURLResponse{ShortUrl: short_url}, nil
}
