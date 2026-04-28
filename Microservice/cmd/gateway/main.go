package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"urlshortener/config"
	"urlshortener/internal/gateway/client"
	gatewayhandler "urlshortener/internal/gateway/handler"
	gatewaymiddleware "urlshortener/internal/gateway/middleware"
)

func main() {
	cfg := config.LoadGateway()

	// Клиент к микросервису генерации ID
	idClient := client.New(cfg.IDServiceURL, cfg.UpstreamTimeout)

	// Роутер
	h := gatewayhandler.New(idClient)
	router := h.NewRouter()

	// Цепочка middleware (применяются снаружи внутрь):
	// Recover → RequestID → RateLimiter → Logger → router
	chain := gatewaymiddleware.Recover(
		gatewaymiddleware.RequestID(
			gatewaymiddleware.RateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)(
				gatewaymiddleware.Logger(router),
			),
		),
	)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      chain,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[GATEWAY] listening on :%s, upstream=%s", cfg.Port, cfg.IDServiceURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	log.Println("[GATEWAY] shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("[GATEWAY] stopped")
}
