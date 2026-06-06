package generator

import (
	"errors"
	"sync"
	"time"
	"urlshortener/internal/dbstorage/pool"
)

const (
	timestampBits  = 28
	datacenterBits = 2
	machineBits    = 1
	sequenceBits   = 11

	MaxDatacenterID = (1 << datacenterBits) - 1 // 3
	MaxMachineID    = (1 << machineBits) - 1    // 1
	MaxSequence     = (1 << sequenceBits) - 1   // 2047

	machineShift    = sequenceBits
	datacenterShift = sequenceBits + machineBits
	timestampShift  = sequenceBits + machineBits + datacenterBits

	ShortenerEpoch   int64 = 1777939200
	MaxTimestampBits int64 = (1 << timestampBits) - 1
)

// IDGenerator — интерфейс генератора. Позволяет подменять реализацию в тестах.
type IDGenerator interface {
	NextID() (int64, error)
}

// Generator — потокобезопасная реализация Snowflake ID.
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
	//	Epoch        int64
	DatacenterID int64
	MachineID    int64
}

func New(cfg *Config, timeEngine pool.DBTime) (*Generator, error) {
	if cfg.DatacenterID < 0 || cfg.DatacenterID > MaxDatacenterID {
		return nil, errors.New("datacenterID must be between 0 and 3")
	}
	if cfg.MachineID < 0 || cfg.MachineID > MaxMachineID {
		return nil, errors.New("machineID must be between 0 and 1")
	}
	epoch := ShortenerEpoch

	return &Generator{
		epoch:         epoch,
		datacenterID:  cfg.DatacenterID,
		machineID:     cfg.MachineID,
		lastTimestamp: -1,
		timeEngine:    timeEngine,
	}, nil
}

func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// now := time.Now().Unix()
	now := g.timeEngine.Now().Unix()

	if now < g.lastTimestamp {
		return 0, errors.New("clock moved backwards, refusing to generate ID")
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
		return 0, errors.New("max time point exceeded")
	}

	id := (timeStamp << timestampShift) |
		(g.datacenterID << datacenterShift) |
		(g.machineID << machineShift) |
		g.sequence

	return id, nil
}

func (g *Generator) waitNextTime(now int64, last int64) int64 {
	// now := time.Now().Unix()
	// now := g.timeEngine.Now().Unix()
	for now <= last {
		/*
			diff := last - now
			time.Sleep(time.Duration(diff) * time.Second)
			now = g.timeEngine.Now().Unix()
		*/
		exactNow := g.timeEngine.Now()
		// Считаем, сколько миллисекунд осталось до конца текущей секунды
		// exactNow.Nanosecond() / 1e6 переводит наносекунды в миллисекунды (0-999)
		msPassed := exactNow.Nanosecond() / int(time.Millisecond)
		msToWait := 1000 - msPassed
		time.Sleep(time.Duration(msToWait+1) * time.Millisecond)

		// Обновляем Unix-время для проверки условия цикла
		now = g.timeEngine.Now().Unix()
	}

	return now
}
