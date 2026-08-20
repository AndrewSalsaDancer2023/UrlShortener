package handler

import (
	"context"
	"log"
	"net"
	"slices"
	"testing"
	"time"
	"urlshortener/config"
	"urlshortener/internal/idgenerator"
	pb "urlshortener/internal/proto/idservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// Задаем размер буфера для in-memory сети
const bufSize = 1024 * 1024
const (
	buffersize = 3 //10
	batchsize  = 3 //250
)

// mockGenerator для изоляции gRPC слоя от реальной логики Snowflake
type mockGenerator struct{}

func (m *mockGenerator) NextBatch(n int) (idgenerator.IDBatch, error) { return []int64{1, 2, 3}, nil }

// panicGenerator — мок генератора, который выбрасывает panic при вызове.
// Если ваш handler.NewHandler вызывает методы генератора или буфера,
// этот мок гарантированно вызовет панику внутри gRPC метода.
type panicGenerator struct{}

func (m *panicGenerator) Push(ctx context.Context, batch idgenerator.IDBatch) error {
	panic("умышленная критическая ошибка для проверки recovery")
}

func (m *panicGenerator) TakeBatch(ctx context.Context) (idgenerator.IDBatch, error) {
	panic("умышленная критическая ошибка для проверки recovery")
}

func (m *panicGenerator) NextBatch(n int) (idgenerator.IDBatch, error) { return []int64{42}, nil }

type workingGenerator struct{}

func (m *workingGenerator) NextBatch(n int) (idgenerator.IDBatch, error) { return []int64{42}, nil }

type blockingGenerator struct{}

func (b *blockingGenerator) NextBatch(n int) (idgenerator.IDBatch, error) {
	// Намертво блокируем горутину продюсера, симулируя зависание
	select {}
}

// Панический мок буфера.
// Подмените типы методов согласно вашей реальной структуре Buffer,
// если они возвращают другие типы (например, метод Pop() или аналогичный)
type panicBuffer struct {
}

// Предположим, ваш gRPC handler вызывает метод Pop() или Get() у буфера.
func (p *panicBuffer) Push(ctx context.Context, batch idgenerator.IDBatch) error {
	panic("умышленная паника вызова Push внутри gRPC хэндлера")
}

func (p *panicBuffer) TakeBatch(ctx context.Context) (idgenerator.IDBatch, error) {
	panic("умышленная паника вызова TakeBatch внутри gRPC хэндлера")
}

// Переопределяем этот метод, чтобы он паниковал внутри gRPC горутины!
func (p *panicBuffer) Pop() (int64, error) {
	panic("умышленная паника внутри gRPC хэндлера")
}

// Тест 1: Проверка интерцептора EnforceDeadlineInterceptor (Негативный сценарий)
/*
func TestEnforceDeadlineInterceptor_MissingDeadline(t *testing.T) {
	// 1. Создаем in-memory листенер
	lis := bufconn.Listen(bufSize)

	// 2. Инициализируем компоненты приложения
	cfg := config.Config{Port: "bufnet"}
	buf := idgenerator.NewBuffer(buffersize, batchsize)
	mockGen := &mockGenerator{}

	app := NewApp(cfg, buf, mockGen, batchsize)

	// Запускаем сервер в фоне
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = app.grpcServer.Serve(lis)
	}()
	defer app.grpcServer.Stop()

	// 3. Создаем gRPC клиент, работающий через in-memory соединение
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewIDServiceClient(conn)

	// 4. Делаем вызов БЕЗ установленного Deadline в контексте
	_, err = client.GetIDBatch(context.Background(), &pb.GetBatchRequest{})

	// 5. Проверяем, что интерцептор перехватил запрос и вернул InvalidArgument
	if err == nil {
		t.Fatal("Ожидалась ошибка от интерцептора, но запрос прошел успешно")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Ожидался статус gRPC ошибки, получено: %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("Ожидался код InvalidArgument, получен: %v", st.Code())
	}
}
*/
func TestEnforceDeadlineInterceptor_MissingDeadline(t *testing.T) {
	lis := bufconn.Listen(bufSize)

	cfg := config.Config{Port: "bufnet_missing"} // Уникальный порт/имя для этого теста
	buf := idgenerator.NewBuffer(buffersize)
	mockGen := &mockGenerator{}

	app := NewApp(cfg, buf, mockGen, batchsize)

	go func() {
		_ = app.grpcServer.Serve(lis)
	}()
	defer app.grpcServer.Stop()

	// Используем выделенный контекст для контроля подключения самого клиента
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer clientCancel()

	// Указываем уникальное имя цели "bufnet_missing", чтобы gRPC не переиспользовал кэш соединений
	conn, err := grpc.NewClient("passthrough:///bufnet_missing",
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			if clientCtx.Err() != nil {
				return nil, clientCtx.Err()
			}
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet_missing: %v", err)
	}
	defer conn.Close()

	client := pb.NewIDServiceClient(conn)

	// Делаем вызов с context.Background() (дедлайна гарантированно нет)
	_, err = client.GetIDBatch(context.Background(), &pb.GetBatchRequest{})

	// ПРОВЕРКА: Ошибка ОБЯЗАТЕЛЬНО должна быть
	if err == nil {
		t.Fatal("Ожидалась ошибка от интерцептора EnforceDeadline, но запрос прошел успешно (код OK)")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Ожидался валидный статус gRPC ошибки, но получено: %v", err)
	}

	// Проверяем, что интерцептор вернул именно InvalidArgument
	if st.Code() != codes.InvalidArgument {
		t.Errorf("Ожидался код ошибки [%v] (InvalidArgument), но сервер вернул [%v]. Сообщение ошибки: %s",
			codes.InvalidArgument, st.Code(), st.Message())
	}
}

// Тест 2: Успешный вызов gRPC метода при соблюдении условий (Happy Path)
func TestEnforceDeadlineInterceptor_WithValidDeadline(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	cfg := config.Config{Port: "bufnet"}
	buf := idgenerator.NewBuffer(buffersize)
	mockGen := &mockGenerator{}

	// Чтобы хэндлер не висел в ожидании ID из пустого буфера,
	// вручную добавим один тестовый ID в буфер перед вызовом
	_ = buf.Push(context.Background(), []int64{42})

	app := NewApp(cfg, buf, mockGen, batchsize)

	go func() { _ = app.grpcServer.Serve(lis) }()
	defer app.grpcServer.Stop()
	/*
		conn, err := grpc.DialContext(context.Background(), "bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)*/
	// Используем выделенный контекст для контроля подключения самого клиента
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer clientCancel()
	// Указываем уникальное имя цели "bufnet_missing", чтобы gRPC не переиспользовал кэш соединений
	conn, err := grpc.NewClient("passthrough:///bufnet_missing",
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			if clientCtx.Err() != nil {
				return nil, clientCtx.Err()
			}
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewIDServiceClient(conn)

	// Создаем контекст С дедлайном (на 1 секунду)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Вызываем метод

	resp, err := client.GetIDBatch(ctx, &pb.GetBatchRequest{})

	// Проверяем, что теперь интерцептор пропустил запрос, и хэндлер вернул ID
	if err != nil {
		t.Fatalf("Неожиданная ошибка при наличии дедлайна: %v", err)
	}

	expected := []int64{42}
	if !slices.Equal(resp.GetIds(), expected) {
		t.Errorf("Ожидался ID 42, получен %v", resp.GetIds())
	}
}

func TestRecoveryInterceptor_CatchPanic3(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	cfg := config.Config{Port: "bufnet_panic"}

	// 1. Создаем панический буфер
	mockBuf := &panicBuffer{}

	// Продюсеру даем рабочий генератор, чтобы ОН не паниковал в фоне
	workingGen := &workingGenerator{}

	// Инициализируем приложение с паническим буфером
	app := NewApp(cfg, mockBuf, workingGen, batchsize)

	// Запускаем сервер напрямую через gRPC, без фонового продюсера,
	// чтобы исключить сторонние паники
	go func() {
		_ = app.grpcServer.Serve(lis)
	}()
	defer app.grpcServer.Stop()

	// Настройка клиента
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer clientCancel()

	conn, err := grpc.NewClient("passthrough:///bufnet_panic_v3",
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			if clientCtx.Err() != nil {
				return nil, clientCtx.Err()
			}
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet_panic: %v", err)
	}
	defer conn.Close()

	client := pb.NewIDServiceClient(conn)

	ctxWithDeadline, reqCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer reqCancel()

	// 2. Делаем вызов. Хэндлер пойдет в наш mockBuf, вызовет метод чтения и запаникует
	_, err = client.GetIDBatch(ctxWithDeadline, &pb.GetBatchRequest{})

	// 3. ПРОВЕРКИ ИНТЕРЦЕПТОРА RECOVERY
	if err == nil {
		t.Fatal("Expected server error due to panic(), but request returned  code OK")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected status gRPC error, received: %v", err)
	}

	// Теперь recovery interceptor обязан поймать эту панику,
	// так как она произошла внутри gRPC потока!
	if st.Code() != codes.Internal {
		t.Errorf("Expected code [Internal], received [%v]. Message: %s", st.Code(), st.Message())
	}

	expectedMsg := "Internal server error"
	if st.Message() != expectedMsg {
		t.Errorf("Expected error message '%s', received '%s'", expectedMsg, st.Message())
	}
}

func TestApp_SuccessfulRunAndGracefulShutdown(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	cfg := config.Config{Port: "bufnet_integration"}

	// Даем абсолютно рабочие моки буфера и генератора
	workingBuf := idgenerator.NewBuffer(buffersize)
	workingGen := &workingGenerator{}

	app := NewApp(cfg, workingBuf, workingGen, batchsize)

	// Создаем контекст, который мы отменим вручную через 200 миллисекунд
	ctx, cancel := context.WithCancel(context.Background())

	// Канал для отслеживания ошибок из App.Run
	runErrChan := make(chan error, 1)

	// 1. Запускаем метод App.Run
	go func() {
		runErrChan <- app.Run(ctx, lis)
	}()

	// Даем серверу и продюсеру немного времени просто поработать вхолостую
	time.Sleep(200 * time.Millisecond)

	// 2. Симулируем приход системного сигнала (отменяем контекст)
	// Это должно спровоцировать автоматический вызов app.Stop() внутри Run
	cancel()

	// 3. Проверяем результат выполнения App.Run
	select {
	case err := <-runErrChan:
		if err != nil {
			t.Errorf("App.Run завершился с ошибкой: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Тест завис! Плавное завершение App.Stop не отработало за 3 секунды")
	}
}

func TestApp_Lifecycle_ServerError(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	// Искусственно закрываем листенер, делая его непригодным для Serve()
	_ = lis.Close()

	cfg := config.Config{Port: "bufnet_fail"}
	app := NewApp(cfg, idgenerator.NewBuffer(buffersize), &workingGenerator{}, 3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErrChan := make(chan error, 1)

	// Запускаем
	go func() {
		runErrChan <- app.Run(ctx, lis)
	}()

	// Проверяем, что App.Run сам мгновенно вернул ошибку и завершился,
	// а не завис в бесконечном ожидании
	select {
	case err := <-runErrChan:
		if err == nil {
			t.Fatal("Ожидалась ошибка запуска gRPC сервера, но App.Run завершился без ошибок")
		}
		log.Printf("Зафиксирована ожидаемая ошибка старта: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Приложение зависло при падении листенера!")
	}
}

func TestApp_Lifecycle_ShutdownTimeout(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	cfg := config.Config{Port: "bufnet_timeout"}

	// Передаем блокирующий генератор, чтобы продюсер завис при закрытии
	app := NewApp(cfg, idgenerator.NewBuffer(buffersize), &blockingGenerator{}, 3)

	// runCtx, stopApp := context.WithCancel(context.Background())
	runCtx, stopApp := context.WithTimeout(context.Background(), 10*time.Second)
	runErrChan := make(chan error, 1)

	go func() {
		runErrChan <- app.Run(runCtx, lis)
	}()

	time.Sleep(100 * time.Millisecond)

	// Инициируем остановку. Начнется Graceful Shutdown.
	// gRPC сервер закроется, но продюсер завис в select{}, поэтому
	// wg.Wait() в методе Stop() будет висеть вечно.
	startShutdown := time.Now()
	stopApp()

	// Наш код App.Stop() устроен так, что через 5 секунд таймаута он должен
	// принудительно прервать ожидание и выйти с ошибкой.
	select {
	case err := <-runErrChan:
		if err == nil {
			t.Error("Ожидалась ошибка таймаута shutdown, но метод вернул nil")
		}

		shutdownDuration := time.Since(startShutdown)
		if shutdownDuration < 5*time.Second {
			t.Errorf("Сервер закрылся слишком быстро (%v), таймаут 5 секунд не отработал", shutdownDuration)
		}
		log.Printf("Приложение успешно совершило Forced Stop за %v", shutdownDuration)

	case <-time.After(6 * time.Second):
		t.Fatal("Критическая ошибка! Таймаут в App.Stop не сработал, приложение зависло в памяти навсегда")
	}
}
