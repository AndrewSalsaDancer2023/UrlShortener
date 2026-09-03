package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"urlshortener/config"

	"urlshortener/internal/dbstorage/pool"
	"urlshortener/internal/idgenerator"
	"urlshortener/internal/idgenerator/handler"
	"urlshortener/utils"
)

const (
	buffersize = 3 //10
	batchsize  = 3 //250
)

func main() {
	// 1. Загружаем конфигурацию
	cfg := config.Load()
	log.SetOutput(os.Stdout)
	//port := os.Getenv("PORT")

	if nodeId, err := utils.ExtractNodeID(); err != nil {
		log.Printf("failed to extract nodeId: %v", err)
		return
	} else {
		datacenterID, machineID := utils.ExtractMachineAndDataCenterID(nodeId)
		log.Printf("starting with mach Id: %d and node Id: %d", datacenterID, machineID)
		cfg.SetDataCenterAndMachineID(datacenterID, machineID)
	}

	port := os.Getenv("PORT")
	if len(port) != 0 {
		cfg.SetPort(port)
	}

	// 2. Настройка gRPC-слоя и Перехватчиков (Middleware)
	// Обязательно закрываем файл при завершении работы всего приложения
	// logFile := utils.CreateLogFile("grpc_server" + cfg.Port + ".log")
	// defer logFile.Close()
	// log.SetOutput(logFile)

	timeEngine := pool.UnixTimeReal{}
	// 3. Создаем генератор, буфер и продюсер
	gen, err := idgenerator.NewIDGenerator(&idgenerator.Config{
		DatacenterID: cfg.DatacenterID,
		MachineID:    cfg.MachineID,
	}, timeEngine)
	if err != nil {
		log.Printf("failed to create generator: %v", err)
		return
	}

	buf := idgenerator.NewBuffer(buffersize)
	app := handler.NewApp(cfg, buf, gen, batchsize)

	// 4. Слушаем порт
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Printf("failed to listen port %s: %v", cfg.Port, err)
		return
	}

	// 5. Отслеживаем системные сигналы ОС
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, lis); err != nil {
		log.Printf("App execution failed: %v", err)
	}
	log.Println("gRPC server stopped")
}

//grpcurl -plaintext -import-path ./internal/proto -proto idservice.proto -d '{}' localhost:50051  generator.IDService.GetNextID

//go run ./cmd/idgenservice/ --port=50051
