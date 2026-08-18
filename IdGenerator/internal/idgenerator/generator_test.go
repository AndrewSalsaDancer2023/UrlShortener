package idgenerator_test

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"urlshortener/internal/dbstorage/pool"
	"urlshortener/internal/idgenerator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"context"
	"fmt"
	"sync/atomic"
	"time"
)

var globalBatch idgenerator.IDBatch

// mockTimeEngine реализует интерфейс pool.DBTime для подмены времени в тестах
type mockTimeEngine struct {
	//	pool.DBTime
	fixedTime time.Time
}

func (m mockTimeEngine) Now() time.Time {
	return m.fixedTime
}

// fastTickingEngine сдвигает время на 1 секунду при каждом обращении,
// чтобы генератор в бенчмарках никогда не упирался в лимит MaxSequence.
type fastTickingEngine struct {
	currentUnix int64
}

func (f *fastTickingEngine) Now() time.Time {
	// Атомарно увеличиваем секунду (для поддержки конкурентных тестов, если понадобятся)
	// В обычном бенчмарке это просто быстро сдвигает время вперед
	f.currentUnix++
	return time.Unix(f.currentUnix, 0)
}

// Потокобезопасный мок времени для параллельных тестов
type safeTickingEngine struct {
	currentUnix int64
}

func (s *safeTickingEngine) Now() time.Time {
	// Атомарно инкрементируем секунду, чтобы избежать race condition в тесте
	val := atomic.AddInt64(&s.currentUnix, 1)
	return time.Unix(val, 0)
}

func newGen(t *testing.T) *idgenerator.Generator {
	t.Helper()
	timeEngine := pool.UnixTimeReal{}
	g, err := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: 0, MachineID: 0}, timeEngine)
	require.NoError(t, err)
	return g
}

// --- тесты конструктора ---

func TestNew_ValidConfig(t *testing.T) {
	cases := []idgenerator.Config{
		{DatacenterID: 0, MachineID: 0},
		{DatacenterID: idgenerator.MaxDatacenterID, MachineID: idgenerator.MaxMachineID},
		{DatacenterID: 1, MachineID: 1},
	}
	timeEngine := pool.UnixTimeReal{}
	for _, cfg := range cases {
		g, err := idgenerator.NewIDGenerator(&cfg, timeEngine)
		assert.NoError(t, err)
		assert.NotNil(t, g)
	}
}

func TestNew_InvalidDatacenterID(t *testing.T) {
	timeEngine := pool.UnixTimeReal{}
	for _, id := range []int64{-1, idgenerator.MaxDatacenterID + 1} {
		_, err := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: id}, timeEngine)
		assert.Error(t, err, "DatacenterID=%d must be rejected", id)
	}
}

func TestNew_InvalidMachineID(t *testing.T) {
	timeEngine := pool.UnixTimeReal{}
	for _, id := range []int64{-1, idgenerator.MaxMachineID + 1} {
		_, err := idgenerator.NewIDGenerator(&idgenerator.Config{MachineID: id}, timeEngine)
		assert.Error(t, err, "MachineID=%d must be rejected", id)
	}
}

// --- тесты NextID ---

func TestNextID_Positive(t *testing.T) {
	g := newGen(t)
	id, err := g.NextID()
	require.NoError(t, err)
	assert.Greater(t, id, int64(0), "ID must be positive (sign bit = 0)")
}

func TestNextID_Monotonic(t *testing.T) {
	g := newGen(t)
	prev, _ := g.NextID()
	for i := 0; i < 10_000; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		assert.Greater(t, id, prev, "IDs must be monotonically increasing")
		prev = id
	}
}

func TestNextID_Unique(t *testing.T) {
	g := newGen(t)
	const n = 10_000
	seen := make(map[int64]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := g.NextID()
		require.NoError(t, err)
		_, dup := seen[id]
		assert.False(t, dup, "duplicate ID: %d", id)
		seen[id] = struct{}{}
	}
}

func TestNextID_Concurrent_Unique(t *testing.T) {
	g := newGen(t)
	const goroutines, perGoroutine = 20, 500

	var mu sync.Mutex
	seen := make(map[int64]struct{}, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]int64, 0, perGoroutine)
			for j := 0; j < perGoroutine; j++ {
				id, err := g.NextID()
				require.NoError(t, err)
				local = append(local, id)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range local {
				_, dup := seen[id]
				assert.False(t, dup, "concurrent duplicate: %d", id)
				seen[id] = struct{}{}
			}
		}()
	}
	wg.Wait()
	assert.Len(t, seen, goroutines*perGoroutine)
}

func BenchmarkNextID(b *testing.B) {
	timeEngine := pool.UnixTimeReal{}
	g, _ := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: 1, MachineID: 1}, timeEngine)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.NextID()
	}
}

func TestCounterGenerator_NextBatch_HappyPath(t *testing.T) {
	timeEngine := pool.UnixTimeReal{}
	gen, _ := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: 1, MachineID: 1}, timeEngine)

	batch, err := gen.NextBatch(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batch) != 5 {
		t.Fatalf("expected batch of 5, got %d", len(batch))
	}

	// Проверяем уникальность внутри батча и монотонность между батчами.
	seen := make(map[int64]bool, 5)
	for _, id := range batch {
		if seen[id] {
			t.Fatalf("duplicate id within batch: %d", id)
		}
		seen[id] = true
	}

	next, err := gen.NextBatch(5)
	if err != nil {
		t.Fatalf("unexpected error on second batch: %v", err)
	}
	for _, id := range next {
		if seen[id] {
			t.Fatalf("id %d repeated across batches", id)
		}
	}
}

// NextBatch(0) не должен возвращать ошибку — это осознанное поведение
// "нечего генерировать", а не сбой.
func TestCounterGenerator_NextBatch_ZeroIsNoop(t *testing.T) {
	timeEngine := pool.UnixTimeReal{}
	gen, _ := idgenerator.NewIDGenerator(&idgenerator.Config{}, timeEngine)

	batch, err := gen.NextBatch(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batch != nil {
		t.Fatalf("expected nil batch for n=0, got %v", batch)
	}
}

func TestMaxIDsInOneSecond_Parallel(t *testing.T) {
	// 1. Инициализируем генератор
	timeEngine := pool.UnixTimeReal{}
	gen, _ := idgenerator.NewIDGenerator(&idgenerator.Config{}, timeEngine)

	// Счетчик успешно сгенерированных ID
	var successCount int64
	// Счетчик ошибок (когда уперлись в лимит 4096 или сработал таймаут)
	var errorCount int64

	// Контекст, который закроется ровно через 1 секунду
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Количество параллельных горутин, имитирующих клиентов
	// Поставьте 100-500 для симуляции высокой конкуренции
	workerCount := 200
	var wg sync.WaitGroup

	// Запускаем воркеры
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					// Секунда прошла, останавливаем горутину
					return
				default:
					// Пытаемся сгенерировать ID
					_, err := gen.NextID()
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						// Даем микро-паузу планировщику Go, чтобы не забить CPU
						// бесконечными ошибками, если метод ушел в waitNextSecond
						time.Sleep(10 * time.Microsecond)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}
		}()
	}

	// Ждем завершения секунды и остановки всех воркеров
	wg.Wait()

	// Выводим результаты
	fmt.Printf("\n=== РЕЗУЛЬТАТЫ НАГРУЗОЧНОГО ТЕСТА ЗА 1 СЕКУНДУ ===\n")
	fmt.Printf("Успешно сгенерировано ID: %d\n", atomic.LoadInt64(&successCount))
	fmt.Printf("Количество отказов/ожиданий: %d\n", atomic.LoadInt64(&errorCount))
	fmt.Printf("==================================================\n\n")
}

// Тест 1: Проверяет, что при превышении MaxTimestampBits возвращается ошибка ErrCounterExhausted
func TestGenerator_NextID_CounterExhausted(t *testing.T) {
	// Инициализируем мок времени значением, которое сильно опережает эпоху.
	// timeStamp = (now - epoch).
	// Нам нужно, чтобы timeStamp был больше, чем MaxTimestampBits (2^28 - 1 = 268435455 секунд).
	// Возьмем смещение: MaxTimestampBits + 100 секунд
	futureUnix := idgenerator.ShortenerEpoch + idgenerator.MaxTimestampBits + 100
	mockTime := mockTimeEngine{
		fixedTime: time.Unix(futureUnix, 0),
	}
	cfg := idgenerator.Config{}
	gen, _ := idgenerator.NewIDGenerator(&cfg, mockTime)

	// Вызываем метод
	_, err := gen.NextID()

	// Проверяем, что ошибка вернулась и это именно ErrCounterExhausted
	if err == nil {
		t.Fatal("Ожидалась ошибка ErrCounterExhausted, но ID был успешно сгенерирован")
	}

	if !errors.Is(err, idgenerator.ErrCounterExhausted) {
		t.Errorf("Ожидалась ошибка [%v], но получена [%v]", idgenerator.ErrCounterExhausted, err)
	}
}

// Тест 2: Проверяет, что в граничном (максимально допустимом) состоянии ошибка НЕ выбрасывается
func TestGenerator_NextID_MaxValidTimestamp(t *testing.T) {
	// Точно граничное значение: timeStamp == MaxTimestampBits
	exactMaxUnix := idgenerator.ShortenerEpoch + idgenerator.MaxTimestampBits
	mockTime := &mockTimeEngine{
		fixedTime: time.Unix(exactMaxUnix, 0),
	}

	cfg := idgenerator.Config{}
	gen, _ := idgenerator.NewIDGenerator(&cfg, mockTime)
	// Вызываем метод
	id, err := gen.NextID()

	// Ошибки быть не должно
	if err != nil {
		t.Fatalf("При граничном значении времени произошла непредвиденная ошибка: %v", err)
	}

	if id == 0 {
		t.Error("Сгенерированный ID не должен быть равен 0")
	}
}

// //////////////////////////////////////////////////////////////////////
// Тест 1: Проверка некорректного аргумента n <= 0
func TestGenerator_NextBatch_InvalidSize(t *testing.T) {
	gen := &idgenerator.Generator{} // Для этого теста зависимости не важны

	// Проверяем 0
	batch, err := gen.NextBatch(0)
	if batch != nil || err != nil {
		t.Errorf("Для n=0 ожидалось (nil, nil), получено: batch=%v, err=%v", batch, err)
	}

	// Проверяем отрицательное число
	batch, err = gen.NextBatch(-5)
	if batch != nil || err != nil {
		t.Errorf("Для n=-5 ожидалось (nil, nil), получено: batch=%v, err=%v", batch, err)
	}
}

// Тест 2: Успешная генерация стандартного пакета ID
func TestGenerator_NextBatch_Success(t *testing.T) {
	mockTime := &mockTimeEngine{fixedTime: time.Unix(idgenerator.ShortenerEpoch+10, 0)}

	cfg := idgenerator.Config{}
	gen, _ := idgenerator.NewIDGenerator(&cfg, mockTime)

	n := 5
	batch, err := gen.NextBatch(n)
	if err != nil {
		t.Fatalf("Неожиданная ошибка: %v", err)
	}

	// Проверяем размер среза
	if len(batch) != n {
		t.Fatalf("Ожидался размер батча %d, получен %d", n, len(batch))
	}

	// Проверяем уникальность ID внутри батча
	if batch[0] == batch[1] {
		t.Error("Идентификаторы внутри батча дублируются")
	}
}

// Тест 3: Проброс ошибки из NextID наружу
func TestGenerator_NextBatch_ErrorPropagation(t *testing.T) {
	// Искусственно создаем время, превышающее MaxTimestampBits
	futureUnix := idgenerator.ShortenerEpoch + idgenerator.MaxTimestampBits + 100
	mockTime := &mockTimeEngine{fixedTime: time.Unix(futureUnix, 0)}

	cfg := idgenerator.Config{}
	gen, _ := idgenerator.NewIDGenerator(&cfg, mockTime)

	// Запрашиваем батч, но NextID() сразу должен вернуть ErrCounterExhausted
	batch, err := gen.NextBatch(10)
	if batch != nil {
		t.Error("Батч должен быть nil при возникновении ошибки")
	}

	if !errors.Is(err, idgenerator.ErrCounterExhausted) {
		t.Errorf("Ожидалась ошибка %v, получена %v", idgenerator.ErrCounterExhausted, err)
	}
}

func BenchmarkGenerator_NextID_Parallel(b *testing.B) {
	// 1. Инициализируем генератор один раз для всего теста
	timeEngine := pool.UnixTimeReal{}
	gen, _ := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: 0, MachineID: 0}, timeEngine)

	// 2. Сбрасываем таймер перед запуском, чтобы время подготовки не учитывалось
	b.ResetTimer()

	// 3. Запускаем параллельный тест
	// b.RunParallel автоматически создаст количество горутин,
	// равное GOMAXPROCS вашего процессора (обычно равно количеству ядер)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.NextID()
			if err != nil {
				// Если во время теста часы ушли назад он завершится с ошибкой
				b.Fatalf("Ошибка генерации ID: %v", err)
			}
		}
	})
}

func BenchmarkGenerator_NextBatch(b *testing.B) {
	// Размеры батчей, которые мы хотим протестировать
	//sizes := []int{10, 100, 1000, 3000}
	sizes := []int{300}

	for _, size := range sizes {
		// b.Run создает изолированный под-бенчмарк для каждого размера
		b.Run(fmt.Sprintf("BatchSize_%d", size), func(b *testing.B) {

			// Инициализируем генератор с быстрым временем
			timeEngine := &fastTickingEngine{currentUnix: idgenerator.ShortenerEpoch + 10}
			gen, _ := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: 0, MachineID: 0}, timeEngine)

			// Сбрасываем таймер, чтобы подготовка структуры не влияла на метрики
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				res, err := gen.NextBatch(size)
				if err != nil {
					b.Fatalf("Ошибка: %v", err)
				}
				globalBatch = res // Гарантируем, что данные используются
			}
		})
	}
}

func BenchmarkGenerator_NextBatch_Parallel(b *testing.B) {
	// Фиксируем размер батча для теста (например, 100 элементов)
	batchSize := 250

	// Инициализируем генератор один раз для всех горутин
	timeEngine := &fastTickingEngine{currentUnix: idgenerator.ShortenerEpoch + 10}
	//timeEngine := &pool.UnixTimeReal{}
	gen, _ := idgenerator.NewIDGenerator(&idgenerator.Config{DatacenterID: 0, MachineID: 0}, timeEngine)

	// Рассчитываем коэффициент для SetParallelism, чтобы получить ровно 20 горутин
	// Формула: GOMAXPROCS * X = 20  =>  X = 20 / GOMAXPROCS
	numCPUs := runtime.GOMAXPROCS(0)
	parallelism := 20 / numCPUs
	if parallelism < 1 {
		parallelism = 1 // Защита, если ядер больше 20
	}

	// Устанавливаем количество параллельных потоков
	b.SetParallelism(parallelism)

	// Сбрасываем таймер перед стартом
	b.ResetTimer()

	// Запуск параллельного штурма мьютекса
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.NextBatch(batchSize)
			if err != nil {
				b.Fatalf("Ошибка генерации батча в параллельном потоке: %v", err)
			}
		}
	})
}
