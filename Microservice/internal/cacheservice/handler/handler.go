package handler

import (
	"context"
	"time"
	srv "urlshortener/internal/cacheservice"
	"urlshortener/internal/cacheservice/config"
	pb "urlshortener/internal/proto/cacheservice"
)

type GRPCHandler struct {
	// Встраиваем обязательную заглушку для обратной совместимости
	pb.UnimplementedUrlCacheServiceServer
	// service      *srv.UrlCacheService
	service      srv.UrlCache
	writeTiemout time.Duration
	readTiemout  time.Duration
}

func New(srv *srv.UrlCacheService, cfg *config.CacheConfig) *GRPCHandler {
	return &GRPCHandler{
		service:      srv,
		writeTiemout: cfg.WriteTimeout,
		readTiemout:  cfg.ReadTimeout,
	}
}

func (h *GRPCHandler) WriteURLPair(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, h.writeTiemout)
	defer cancel() // Обязательно освобождаем ресурсы в конце

	id, err := h.service.WriteURLPair(timeoutCtx, req.GetShortUrl(), req.GetLongUrl())
	if err != nil {
		return nil, err
	}

	return &pb.CreateResponse{ShortUrl: id}, nil
}

func (h *GRPCHandler) GetLongURL(ctx context.Context, req *pb.GetLongURLRequest) (*pb.GetLongURLResponse, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, h.readTiemout)
	defer cancel() // Обязательно освобождаем ресурсы в конце

	longURL, err := h.service.GetLongURL(timeoutCtx, req.GetShortUrl())
	if err != nil {
		return nil, err
	}
	return &pb.GetLongURLResponse{LongUrl: longURL}, nil
}

func (h *GRPCHandler) GetShortURL(ctx context.Context, req *pb.GetShortURLRequest) (*pb.GetShortURLResponse, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, h.readTiemout)
	defer cancel() // Обязательно освобождаем ресурсы в конце

	short_url, err := h.service.GetShortURL(timeoutCtx, req.ShortUrl, req.LongUrl)
	if err != nil {
		return nil, err
	}
	return &pb.GetShortURLResponse{ShortUrl: short_url}, nil
}
