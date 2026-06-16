package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"sync"
	"syscall"
	"time"
	"urlshortener/internal/cacheservice"
	pb "urlshortener/internal/proto/cacheservice"

	srv "urlshortener/internal/cacheservice"
	config "urlshortener/internal/cacheservice/config"

	"urlshortener/utils"

	"google.golang.org/grpc"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

type CacheService struct {
	config *config.CacheConfig
	// logFile      *os.File
	cache        srv.UrlCache
	grpcServer   *grpc.Server
	healthServer *health.Server
	listener     net.Listener
}

func (s *CacheService) Config() *config.CacheConfig {
	if s.config == nil {
		s.config = config.GetURLCacheConfig()
	}
	return s.config
}

/*
	func (s *CacheService) LogFile() *os.File {
		if s.logFile == nil {
			s.logFile = utils.CreateLogFile("grpc_server" + s.Config().Port + ".log")
		}

		return s.logFile
	}
*/
func (s *CacheService) Cache() srv.UrlCache {
	if s.cache == nil {
		s.cache = cacheservice.New(s.Config())
	}

	return s.cache
}

func (s *CacheService) GRPCServer() *grpc.Server {
	if s.grpcServer == nil {
		s.grpcServer = utils.CreateGRPCServer(utils.LoggerOpts, utils.RecoveryOpts)

		grpcHandler := New(s.Cache(), s.Config())
		pb.RegisterUrlCacheServiceServer(s.grpcServer, grpcHandler)
	}

	return s.grpcServer
}

func (s *CacheService) HealthServer() *health.Server {
	if s.healthServer == nil {
		s.healthServer = health.NewServer()
		// Устанавливаем статус SERVING
		s.healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
		healthgrpc.RegisterHealthServer(s.GRPCServer(), s.healthServer)
	}

	return s.healthServer
}

func (s *CacheService) Listener() (net.Listener, error) {

	if s.listener == nil {
		lis, err := net.Listen("tcp", ":"+s.Config().Port)
		if err != nil {
			return nil, fmt.Errorf("failed to listen port %s: %w", s.Config().Port, err)
		}
		s.listener = lis
	}

	return s.listener, nil
}

func (a *CacheService) Close() {
	// ////////////////////////////////////
	var wg sync.WaitGroup
	// Общий таймаут на закрытие каждого ресурса — 5 секунд
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	var errBuf utils.SafeErrorBuffer
	// 1. Закрываем соединение с grpc
	utils.ShutdownResourceParallel(shutdownCtx, &wg, &errBuf, "connection listener", func() error {
		list, _ := a.Listener()
		if list != nil {
			return list.Close()
		}
		return nil //соединение не открыто, закрывать нечего
	})

	// 2. Закрываем Redis (в том же самом контексте или создав новый)
	utils.ShutdownResourceParallel(shutdownCtx, &wg, &errBuf, "connection listener", func() error {
		if a.Cache() != nil {
			a.Cache().Close()
		}
		return nil
	})

	wg.Wait()
	if len(errBuf.GetErrors()) != 0 {
		log.Printf("Errors during closing %s", errors.Join(errBuf.GetErrors()...))
	} else {
		log.Println("All low-level resources closed.")
	}
}

func (a *CacheService) Run(ctx context.Context) error {

	err := a.Cache().TryConnect()
	if err != nil {
		return fmt.Errorf("Unable connect to cache service: %w", err)
	}

	appCtx, appCancel := context.WithCancel(ctx)
	defer appCancel()

	// Запуск фонового чекера
	go utils.StartRedisHealthCheck(appCtx, a.HealthServer(), a.Cache())

	serverErrors := make(chan error, 1)
	go func() {
		listener, err := a.Listener()
		if err != nil {
			serverErrors <- fmt.Errorf("port listening error: %w", err)
		}
		log.Printf("gRPC server listening on :%s", a.Config().Port)
		if err := a.grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErrors <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return err

	case <-ctx.Done():
		log.Printf("ctx cancelled")

	case sig := <-stop:
		log.Printf("Received signal %v, starting Graceful Shutdown...", sig)

		a.healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
		appCancel() // Стопаем фоновый чекер

		// Таймаут на завершение запросов
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		go func() {
			<-shutdownCtx.Done()
			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				log.Println("Timeout expired. Forced Stop.")
				a.grpcServer.Stop()
			}
		}()

		a.grpcServer.GracefulStop()

		log.Println("gRPC server stopped gracefully")
	}

	return nil
}
