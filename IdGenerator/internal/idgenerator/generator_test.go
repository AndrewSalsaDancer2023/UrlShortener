package idgenerator

import (
	"errors"
	"testing"
	"time"
)

// mockTimeEngine позволяет вручную управлять временем в тестах
type mockTimeEngine struct {
	fixedTime time.Time
}

func (m *mockTimeEngine) Now() time.Time {
	return m.fixedTime
}

// TestGenerator_New_InvalidConfig проверяет валидацию параметров при создании
func TestGenerator_New_InvalidConfig(t *testing.T) {
	timeMock := &mockTimeEngine{fixedTime: time.Unix(ShortenerEpoch+10, 0)}

	_, err := NewIDGenerator(&Config{DatacenterID: 4, MachineID: 0}, timeMock)
	if !errors.Is(err, ErrDataCenterID) {
		t.Errorf("Expected error %v, got %v", ErrDataCenterID, err)
	}

	_, err = NewIDGenerator(&Config{DatacenterID: 0, MachineID: 2}, timeMock)
	if !errors.Is(err, ErrMachineID) {
		t.Errorf("Expected error %v, got %v", ErrMachineID, err)
	}
}

// TestGenerator_NextBatch_Success проверяет корректность генерации и битовой маски ID
func TestGenerator_NextBatch_Success(t *testing.T) {
	// Устанавливаем время: ровно через 5 секунд после ShortenerEpoch
	targetUnix := ShortenerEpoch + 5
	timeMock := &mockTimeEngine{fixedTime: time.Unix(targetUnix, 0)}

	cfg := &Config{DatacenterID: 2, MachineID: 1}
	gen, err := NewIDGenerator(cfg, timeMock)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	batch, err := gen.NextBatch(3)
	if err != nil {
		t.Fatalf("NextBatch returned an error: %v", err)
	}

	if len(batch) != 3 {
		t.Fatalf("Expected a batch of length 3, received: %d", len(batch))
	}

	// Рассчитаем ожидаемый ID для первого элемента вручную на базе битовых сдвигов
	expectedTimeDelta := int64(5) // targetUnix - ShortenerEpoch
	expectedBaseID := (expectedTimeDelta << timestampShift) |
		(int64(2) << datacenterShift) |
		(int64(1) << machineShift)

	// Проверяем первый ID (sequence равен 0)
	if batch[0] != expectedBaseID {
		t.Errorf("Invalid format for first ID. Expected: %d, Received: %d", expectedBaseID, batch[0])
	}

	// Проверяем второй ID (sequence равен 1)
	if batch[1] != (expectedBaseID | 1) {
		t.Errorf("Invalid format for second ID. Expected: %d, Received: %d", expectedBaseID|1, batch[1])
	}
}

// TestGenerator_NextBatch_ClockBackward проверяет реакцию на сдвиг часов назад
func TestGenerator_NextBatch_ClockBackward(t *testing.T) {
	timeMock := &mockTimeEngine{fixedTime: time.Unix(ShortenerEpoch+10, 0)}
	gen, _ := NewIDGenerator(&Config{DatacenterID: 0, MachineID: 0}, timeMock)

	// Сначала генерируем батч при корректном времени
	_, _ = gen.NextBatch(1)

	// Сдвигаем время назад на 5 секунд
	timeMock.fixedTime = time.Unix(ShortenerEpoch+5, 0)

	_, err := gen.NextBatch(1)
	if !errors.Is(err, ErrClockBackward) {
		t.Errorf("Expected error %v, got %v", ErrClockBackward, err)
	}
}

// TestGenerator_NextBatch_SequenceOverflow проверяет уход на следующую секунду при переполнении sequence
func TestGenerator_NextBatch_SequenceOverflow(t *testing.T) {
	startTime := time.Unix(ShortenerEpoch+10, 0)
	timeMock := &mockTimeEngine{fixedTime: startTime}
	gen, _ := NewIDGenerator(&Config{DatacenterID: 0, MachineID: 0}, timeMock)

	// Выставляем sequence в пред-максимальное состояние искусственно
	gen.lastTimestamp = ShortenerEpoch + 10
	gen.sequence = MaxSequence - 1 // 2046

	// Запускаем горутину, которая передвинет время вперед через мгновение,
	// чтобы цикл внутри waitNextTime смог завершиться
	go func() {
		time.Sleep(10 * time.Millisecond)
		timeMock.fixedTime = time.Unix(ShortenerEpoch+11, 0) // Передвигаем время на +1 сек
	}()

	// Пытаемся взять батч размером 5 элементов.
	// Так как 2046 + 5 > 2047, генератор должен зависнуть в waitNextTime и выйти на секунде +11
	batch, err := gen.NextBatch(5)
	if err != nil {
		t.Fatalf("Overflow error: %v", err)
	}

	// У первого элемента в новом батче на новой секунде sequence должен сброситься в 0
	expectedTimeDelta := int64(11) // Новая секунда
	expectedBaseID := expectedTimeDelta << timestampShift

	if batch[0] != expectedBaseID {
		t.Errorf("After waiting for a new second, the sequence did not reset to 0. Expected: %d, Received: %d", expectedBaseID, batch[0])
	}
}

// TestGenerator_NextBatch_CounterExhausted проверяет ошибку при превышении емкости бит времени
func TestGenerator_NextBatch_CounterExhausted(t *testing.T) {
	// Сдвигаем время далеко вперед за пределы вместимости 28 бит
	tooFarUnix := ShortenerEpoch + MaxTimestampBits + 100
	timeMock := &mockTimeEngine{fixedTime: time.Unix(tooFarUnix, 0)}

	gen, _ := NewIDGenerator(&Config{DatacenterID: 0, MachineID: 0}, timeMock)

	_, err := gen.NextBatch(1)
	if !errors.Is(err, ErrCounterExhausted) {
		t.Errorf("Expected error %v, got %v", ErrCounterExhausted, err)
	}
}

// BenchmarkNextBatch_1160 замеряет скорость генерации батча из 1160 элементов
func BenchmarkNextBatch_1160(b *testing.B) {
	timeMock := &mockTimeEngine{fixedTime: time.Unix(ShortenerEpoch+10, 0)}
	cfg := &Config{DatacenterID: 1, MachineID: 1}
	gen, _ := NewIDGenerator(cfg, timeMock)

	// Сбрасываем таймер перед запуском цикла, чтобы не учитывать время инициализации
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Искусственно сдвигаем время вперед в моке, чтобы генератор
		// не зависал в цикле ожидания новой секунды при частых вызовах
		timeMock.fixedTime = timeMock.fixedTime.Add(time.Second)

		_, _ = gen.NextBatch(1160)
	}
}

// BenchmarkNextBatch_2048 замеряет скорость генерации батча из 2048 элементов
func BenchmarkNextBatch_2048(b *testing.B) {
	timeMock := &mockTimeEngine{fixedTime: time.Unix(ShortenerEpoch+10, 0)}
	cfg := &Config{DatacenterID: 1, MachineID: 1}
	gen, _ := NewIDGenerator(cfg, timeMock)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		timeMock.fixedTime = timeMock.fixedTime.Add(time.Second)

		_, _ = gen.NextBatch(2048)
	}
}
