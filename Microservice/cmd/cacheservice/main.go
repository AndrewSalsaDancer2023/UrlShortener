package main

import (
	"context"
	"log"
	"urlshortener/internal/cacheservice/handler"
	"urlshortener/utils"
)

func main() {
	// Инициализируем приложение
	logFile := utils.CreateLogFile("grpc_server_cache" + ".log")
	log.SetOutput(logFile)
	defer logFile.Close()
	var service handler.CacheService

	// defer Close гарантирует, что ресурсы закроются ПОСЛЕ завершения Run
	defer service.Close()

	// Запуск рантайма
	if err := service.Run(context.Background()); err != nil {
		log.Fatalf("Критическая ошибка рантайма: %v", err)
	}
}

//for debugging purpose: sudo lsof -i :5057
//sudo kill 1234
/*
grpcurl -plaintext -import-path ./internal/proto -proto cacheservice.proto -d '{"short_url": 12345, "long_url": "https://google.com"}' localhost:50057 cacheservice.UrlCacheService.WriteURLPair

grpcurl -plaintext -import-path ./internal/proto -proto cacheservice.proto -d '{"short_url": 12345}' localhost:50057 cacheservice.UrlCacheService.GetLongURL

grpcurl -plaintext -import-path ./internal/proto -proto cacheservice.proto -d '{"long_url": "https://google.com"}' localhost:50057 cacheservice.UrlCacheService.GetShortURL
*/
