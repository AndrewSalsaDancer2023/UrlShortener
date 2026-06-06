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
	grpclient "urlshortener/internal/gateway/client/grpc"
	"urlshortener/utils"

	//	client "urlshortener/internal/gateway/client/http"
	"urlshortener/internal/base62"
	gatewayhandler "urlshortener/internal/gateway/handler"
	gatewaymiddleware "urlshortener/internal/gateway/middleware"
	/*
	   "google.golang.org/grpc"
	   "google.golang.org/grpc/credentials/insecure"
	   "google.golang.org/grpc/resolver"
	   "google.golang.org/grpc/resolver/manual"
	*/)

func main() {
	cfg := config.LoadGateway()

	/////////////////////////////////////////////////////////////////
	// 1. Список адресов ваших генераторов (задаются при старте)
	//serverAddrs := "ipv4:///127.0.0.1:50051,127.0.0.1:50052"
	/*
	   	serviceConfig := `{
	               "loadBalancingConfig": [
	                       {
	                           "round_robin": {}
	                       }
	                   ],
	               "healthCheckConfig": {
	                       "serviceName": ""
	               }
	       }` */

	serviceConfig, err := utils.ReadJSONFile(utils.ConfigPath)
	if err != nil {
		log.Fatalf("failed to open balance config: %s: %v", utils.ConfigPath, err)
	}

	logFile, err := os.OpenFile(utils.GateWayLogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file %s: %v", utils.GateWayLogFileName, err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	/////////////////////////////////////////////////////////////////
	// Клиент к микросервису генерации ID
	idServiceConfig := grpclient.ClientConfig{
		Scheme: "idgen-cluster",
		Path:   "id-service-endpoints",
		Addrs: []string{
			"127.0.0.1:50051",
			"127.0.0.1:50052",
		},
		ServiceConfig: serviceConfig,
	}

	idClient, err := grpclient.NewGenIDClient(idServiceConfig)
	if err != nil {
		log.Fatalf("ID client creation error: %v", err)
	}
	defer idClient.Close() // Метод Close() доступен автоматически из BaseClient

	// Клиент к микросервису сокращения URL
	urlShortenConfig := grpclient.ClientConfig{
		Scheme: "urlshorten-cluster",
		Path:   "shorten-service-endpoints",
		Addrs: []string{
			"127.0.0.1:50053",
			//"127.0.0.1:50054",
		},
		ServiceConfig: serviceConfig,
	}

	urlShortenClient, err := grpclient.NewURLShortenClient(urlShortenConfig)
	if err != nil {
		log.Fatalf("client url shorten creation error: %v", err)
	}
	defer urlShortenClient.Close()

	// Клиент к микросервису перенаправления на исходный URL
	urlClientConfig := grpclient.ClientConfig{
		Scheme: "urlrestore-cluster",
		Path:   "restore-service-endpoints",
		Addrs: []string{
			"127.0.0.1:50054",
			//		"127.0.0.1:50055",
			//		"127.0.0.1:50056",
		},
		ServiceConfig: serviceConfig,
	}

	urlRestoreClient, err := grpclient.NewURLRestorerClient(urlClientConfig)
	if err != nil {
		log.Fatalf("url restore client creation error: %v", err)
	}
	defer urlRestoreClient.Close()

	// Клиент к микросервису сокращения URL
	cacheClientConfig := grpclient.ClientConfig{
		Scheme: "urlcache-cluster",
		Path:   "cache-service-endpoints",
		Addrs: []string{
			"127.0.0.1:50057",
			"127.0.0.1:50058",
		},
		ServiceConfig: serviceConfig,
	}

	urlCacheClient, err := grpclient.NewURLCacheClient(cacheClientConfig)
	if err != nil {
		log.Fatalf("url cache client creation error: %v", err)
	}
	defer urlRestoreClient.Close()
	converter := base62.NewEncoder()
	// Роутер
	h := gatewayhandler.New(converter, idClient, urlShortenClient, urlRestoreClient, urlCacheClient)
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
	// idClient.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("[GATEWAY] stopped")
}

/*
curl -X POST http://localhost:9090/api/v1/data/shorten \
     -H "Content-Type: application/json" \
     -d '{"longUrl":"https://www.someurl.com"}'

curl -X GET http://localhost:9090/api/v1/short/qwert12 \
     -H "Content-Type: application/json"
*/
