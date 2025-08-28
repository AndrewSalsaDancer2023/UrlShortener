package main

import (
	"log"
	"net/http"
	"time"

	"urlshortener.com/engine/internal/controller"
	httphandler "urlshortener.com/engine/internal/handler"
	"urlshortener.com/gateway/pkg/config"
)

func main() {
	ctrl := controller.New()
	h := httphandler.New(ctrl)
	cfg := config.GetConfig()
	log.Println("Starting short url creation generation service:  " + cfg.EngineURL)
	srv := &http.Server{Handler: httphandler.SetupRouter(h),
		Addr:         cfg.EngineURL,
		WriteTimeout: 3 * time.Second,
		ReadTimeout:  3 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
