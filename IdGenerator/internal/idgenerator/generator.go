package idgenerator

import (
	"errors"
	"sync"
	"time"
	"urlshortener/internal/dbstorage/pool"
)

var (
	ErrCounterExhausted = errors.New("idgen: counter space exhausted")
	ErrDataCenterID     = errors.New("datacenterID must be between 0 and 3")
	ErrMachineID        = errors.New("machineID must be between 0 and 1")
	ErrClockBackward    = errors.New("clock moved backwards, refusing to generate ID")
	ErrInvalidBatchSize = errors.New("Invalid batch size specified")
)

const (
	//размеры полей в битах
	timestampBits  = 28
	datacenterBits = 2
	machineBits    = 1
	sequenceBits   = 11
	//максимальные значения полей
	MaxDatacenterID        = (1 << datacenterBits) - 1 // 3
	MaxMachineID           = (1 << machineBits) - 1    // 1
	MaxSequence            = (1 << sequenceBits) - 1   // 2047
	MaxTimestampBits int64 = (1 << timestampBits) - 1  // 2^28-1
	//смещения полей
	machineShift    = sequenceBits
	datacenterShift = sequenceBits + machineBits
	timestampShift  = sequenceBits + machineBits + datacenterBits
	//временная метка начал работы с сервисом
	ShortenerEpoch int64 = 1777939200
)

type IDBatch []int64

// IDGenerator — интерфейс генератора. Позволяет подменять реализацию в тестах.
type IDGenerator interface {
	NextID() (int64, error)
}

type BatchGenerator interface {
	NextBatch(n int) (IDBatch, error)
}

// Generator для потокобезопасной реализация Snowflake ID.
type Generator struct {
	mu            sync.Mutex
	epoch         int64
	datacenterID  int64
	machineID     int64
	sequence      int64
	lastTimestamp int64
	timeEngine    pool.DBTime
}

type Config struct {
	DatacenterID int64
	MachineID    int64
}

func NewIDGenerator(cfg *Config, timeEngine pool.DBTime) (*Generator, error) {
	if cfg.DatacenterID < 0 || cfg.DatacenterID > MaxDatacenterID {
		return nil, ErrDataCenterID
	}
	if cfg.MachineID < 0 || cfg.MachineID > MaxMachineID {
		return nil, ErrMachineID
	}

	return &Generator{
		epoch:         ShortenerEpoch,
		datacenterID:  cfg.DatacenterID,
		machineID:     cfg.MachineID,
		lastTimestamp: -1,
		timeEngine:    timeEngine,
	}, nil
}

func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.timeEngine.Now().Unix()

	if now < g.lastTimestamp {
		return 0, ErrClockBackward
	}
	if now == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & MaxSequence
		if g.sequence == 0 {
			now = g.waitNextTime(now, g.lastTimestamp)
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = now
	timeStamp := now - g.epoch
	if timeStamp > MaxTimestampBits {
		return 0, ErrCounterExhausted
	}

	id := (timeStamp << timestampShift) |
		(g.datacenterID << datacenterShift) |
		(g.machineID << machineShift) |
		g.sequence

	return id, nil
}

func (g *Generator) waitNextTime(now int64, last int64) int64 {

	for now <= last {
		exactNow := g.timeEngine.Now()
		// Считаем, сколько миллисекунд осталось до конца текущей секунды
		msPassed := exactNow.Nanosecond() / int(time.Millisecond)
		msToWait := 1000 - msPassed
		time.Sleep(time.Duration(msToWait+1) * time.Millisecond)

		// Обновляем Unix-время для проверки условия цикла
		now = g.timeEngine.Now().Unix()
	}

	return now
}

func (g *Generator) NextBatch(n int) (IDBatch, error) {
	if n <= 0 {
		return nil, ErrInvalidBatchSize
	}

	// 1. Выделяем память до блокировки мьютекса,
	// чтобы не держать Lock во время выделения памяти рантаймом Go
	batch := make(IDBatch, n)

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.timeEngine.Now().Unix()

	if now < g.lastTimestamp {
		return nil, ErrClockBackward
	}

	// 2. Если мы находимся в рамках той же секунды, что и lastTimestamp, то
	//  проверяем, влезает ли весь батч в лимит
	if now == g.lastTimestamp {
		// Проверяем, не превысит ли g.sequence + n максимальный лимит 2047
		if g.sequence+int64(n) > MaxSequence {
			// Если батч не влезает целиком, ждем следующую секунду
			now = g.waitNextTime(now, g.lastTimestamp)
			g.sequence = 0
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = now
	timeStamp := now - g.epoch
	if timeStamp > MaxTimestampBits {
		return nil, ErrCounterExhausted
	}

	// Вычисляем базовую часть ID (время, датацентр, машина) один раз
	baseID := (timeStamp << timestampShift) |
		(g.datacenterID << datacenterShift) |
		(g.machineID << machineShift)

	// 3. Заполняем батч.
	for i := range n {
		batch[i] = baseID | g.sequence
		g.sequence++
	}

	return batch, nil
}
