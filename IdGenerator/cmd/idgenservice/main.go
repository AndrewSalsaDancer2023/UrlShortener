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

	// 2. Настройка gRPC-слоя и Перехватчиков (Middleware)
	// Обязательно закрываем файл при завершении работы всего приложения*/
	logFile := utils.CreateLogFile("grpc_server" + cfg.Port + ".log")
	defer logFile.Close()
	log.SetOutput(logFile)

	timeEngine := pool.UnixTimeReal{}
	// 3. Создаем генератор, буфер и продюсер
	gen, err := idgenerator.NewIDGenerator(&idgenerator.Config{
		DatacenterID: cfg.DatacenterID,
		MachineID:    cfg.MachineID,
	}, timeEngine)
	if err != nil {
		log.Fatalf("failed to create generator: %v", err)
	}

	buf := idgenerator.NewBuffer(buffersize)
	app := handler.NewApp(cfg, buf, gen, batchsize)

	// Слушаем порт
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("failed to listen port %s: %v", cfg.Port, err)
	}

	// Отслеживаем системные сигналы ОС
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, lis); err != nil {
		log.Fatalf("App execution failed: %v", err)
	}
	log.Println("gRPC server stopped")
}

//grpcurl -plaintext -import-path ./internal/proto -proto idservice.proto -d '{}' localhost:50051  generator.IDService.GetNextID

//go run ./cmd/idgenservice/ --port=50051

// // 1. Загружаем конфигурацию
// cfg := config.Load()

// // 2. Настройка gRPC-слоя и Перехватчиков (Middleware)
// // Обязательно закрываем файл при завершении работы всего приложения*/
// logFile := utils.CreateLogFile("grpc_server" + cfg.Port + ".log")
// defer logFile.Close()
// log.SetOutput(logFile)

// timeEngine := pool.UnixTimeReal{}
// // 3. Создаем генератор, буфер и продюсер
// gen, err := idgenerator.NewIDGenerator(&idgenerator.Config{
// 	DatacenterID: cfg.DatacenterID,
// 	MachineID:    cfg.MachineID,
// }, timeEngine)
// if err != nil {
// 	log.Fatalf("failed to create generator: %v", err)
// }

// buf := idgenerator.NewBuffer(buffersize, batchsize)

// producer := idgenerator.NewProducer(gen, buf, batchsize)

// //4. Создаем контекст, который отменится при получении сигналов от ОС
// ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
// defer stop()
// var wg sync.WaitGroup

// // 4. Запускаем горутину для продюсера
// wg.Add(1)
// go func() {
// 	defer wg.Done() // Уменьшаем счетчик, когда Run() завершится
// 	producer.Run(ctx)
// }()

// loggerOpts := []logging.Option{
// 	logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
// }
// grpcLogger := logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
// 	log.Printf("[%s] %s %v", utils.CreateDebugLevelString(lvl), msg, fields)
// })

// //5. Настройка Recover Interceptor (перехват panic)
// recoveryOpts := []recovery.Option{
// 	recovery.WithRecoveryHandler(func(p any) (err error) {
// 		log.Printf("Перехвачена критическая ошибка (panic): %v", p)
// 		return status.Errorf(codes.Internal, "Внутренняя ошибка сервера")
// 	}),
// }

// //6. Создаем gRPC сервер и объединяем перехватчики в цепочку (Слева направо: Recover -> Logger -> Router)
// gRPCServer := grpc.NewServer(
// 	grpc.ChainUnaryInterceptor(
// 		recovery.UnaryServerInterceptor(recoveryOpts...),
// 		utils.EnforceDeadlineInterceptor(),
// 		logging.UnaryServerInterceptor(grpcLogger, loggerOpts...),
// 	),
// )

// // 7. Регистрация вашего обработчика (вместо h.NewRouter())
// grpcHandler := handler.NewHandler(buf)
// pb.RegisterIDServiceServer(gRPCServer, grpcHandler)

// //9. gRPC работает поверх чистого TCP соединения, открываем порт
// lis, err := net.Listen("tcp", ":"+cfg.Port)
// if err != nil {
// 	log.Fatalf("failed to listen port %s: %v", cfg.Port, err)
// }

// //вызов gRPCServer.Serve(lis) блокирующий, поэтому запускаем его в отдельной горутине
// go func() {
// 	log.Printf("gRPC server listening on :%s", cfg.Port)
// 	if err := gRPCServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
// 		log.Fatalf("gRPC server error: %v", err)
// 	}
// }()

// <-ctx.Done()
// log.Println("shutting down gRPC server...")

// //10. Даем горутине время на корректное завершение (таймаут на graceful shutdown)
// shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// defer cancel()

// // 11. Запускаем GracefulStop в ОТДЕЛЬНОЙ горутине, чтобы он не блокировал main
// // У gRPC есть встроенный метод GracefulStop().
// // Он блокирует поток, ждет завершения всех активных RPC-запросов и закрывает сервер.
// // Привязывать контекст с таймаутом вручную здесь не требуется.
// grpcDone := make(chan struct{})
// go func() {
// 	gRPCServer.GracefulStop()
// 	close(grpcDone)
// }()

// // 12. Создаем канал для отслеживания завершения всех горутин
// producerDone := make(chan struct{})
// go func() {
// 	wg.Wait()
// 	close(producerDone)
// }()

// select {
// case <-shutdownCtx.Done():
// 	// Время вышло, а кто-то (gRPC или Продюсер) ещё не закрылся
// 	log.Println("Shutdown timeout exceeded! Forcing stop...")
// 	gRPCServer.Stop() // Вот теперь это оправданно — мы спасаем сервер от зависания

// case <-grpcDone:
// 	// gRPC сервер ПЛАВНО завершил все запросы и сам закрылся.
// 	// Теперь проверяем, успел ли выйти продюсер (он должен быть уже закрыт)
// 	select {
// 	case <-producerDone:
// 		log.Println("All systems cleanly exited")
// 	case <-shutdownCtx.Done():
// 		log.Println("Warning: Producer shutdown timed out")
// 	}

// }

// log.Println("gRPC server stopped")
