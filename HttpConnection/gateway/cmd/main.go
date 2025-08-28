package main

import (
	"log"
	"net/http"
	"time"

	cacheagateway "urlshortener.com/gateway/internal/cache"
	"urlshortener.com/gateway/internal/controller"
	enginegateway "urlshortener.com/gateway/internal/engine"
	httphandler "urlshortener.com/gateway/internal/handler"
	"urlshortener.com/gateway/pkg/config"
)

func main() {

	cfg := config.GetConfig()
	cacheGateway := cacheagateway.New(cfg.CacheURL, cfg.ShortURLPath)

	engineGateway := enginegateway.New(cfg.EngineURL, cfg.ShortURLPath)

	ctrl := controller.New(cacheGateway, engineGateway)
	handler := httphandler.New(ctrl)
	log.Println("Starting gateway service:  " + cfg.GatewayURL)
	srv := &http.Server{Handler: handler.SetupRouter(),
		Addr:         cfg.GatewayURL,
		WriteTimeout: 3 * time.Second,
		ReadTimeout:  3 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
