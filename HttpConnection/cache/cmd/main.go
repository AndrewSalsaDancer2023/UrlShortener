package main

import (
	"log"
	"net/http"
	"time"

	"urlshortener.com/cache/internal/controller"
	"urlshortener.com/cache/internal/engine"
	httphandler "urlshortener.com/cache/internal/handler"
	"urlshortener.com/gateway/pkg/config"
)

func main() {
	cache := engine.New()
	ctrl := controller.New(cache)
	h := httphandler.New(ctrl)
	cfg := config.GetConfig()
	log.Println("Starting url caching service:  " + cfg.CacheURL)

	srv := &http.Server{Handler: httphandler.SetupRouter(h),
		Addr:         cfg.CacheURL,
		WriteTimeout: 3 * time.Second,
		ReadTimeout:  3 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
