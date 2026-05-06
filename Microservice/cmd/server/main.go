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
	"urlshortener/internal/base62"
	"urlshortener/internal/eventbus"
	"urlshortener/internal/generator"
	"urlshortener/internal/handler"
	"urlshortener/internal/middleware"
	"urlshortener/internal/service"
)

func main() {
	// 1. Конфигурация
	cfg := config.Load()

	// 2. Инфраструктура: EventBus
	bus := eventbus.New()
	eventbus.RegisterDefaultSubscribers(bus)

	// 3. Низкоуровневые зависимости
	gen, err := generator.New(generator.Config{
		//		Epoch:        cfg.Epoch,
		DatacenterID: cfg.DatacenterID,
		MachineID:    cfg.MachineID,
	})
	if err != nil {
		log.Fatalf("failed to create generator: %v", err)
	}

	encoder := base62.New()

	// 4. Сервисный слой — внедряем зависимости через конструктор
	svc := service.New(gen, encoder, bus)

	// 5. HTTP-слой: handler создаёт gorilla/mux роутер
	h := handler.New(svc)
	router := h.NewRouter()

	// Оборачиваем в middleware (цепочка: Recover → Logger → router)
	chain := middleware.Recover(middleware.Logger(router))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      chain,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 6. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("stopped")
}
