package generator

import (
	"errors"
	"sync"
	"time"
)

const (
	// TwitterEpoch — стартовая эпоха Twitter Snowflake: Nov 04, 2010, 01:42:54 UTC
	TwitterEpoch int64 = 1288834974657

	// Размеры полей (биты)
	timestampBits  = 41
	datacenterBits = 5
	machineBits    = 5
	sequenceBits   = 12

	// Максимальные значения полей: (1 << N) - 1 даёт N единичных бит подряд
	MaxDatacenterID = (1 << datacenterBits) - 1 // 31
	MaxMachineID    = (1 << machineBits) - 1    // 31
	MaxSequence     = (1 << sequenceBits) - 1   // 4095

	// Сдвиги для компоновки ID
	machineShift    = sequenceBits                                // 12
	datacenterShift = sequenceBits + machineBits                  // 17
	timestampShift  = sequenceBits + machineBits + datacenterBits // 22
)

// Generator — потокобезопасный генератор Snowflake ID.
type Generator struct {
	mu            sync.Mutex
	epoch         int64 // кастомная эпоха в миллисекундах (Unix ms)
	datacenterID  int64
	machineID     int64
	sequence      int64
	lastTimestamp int64
}

// Config задаёт параметры генератора.
type Config struct {
	// Epoch — начало отсчёта в миллисекундах (Unix ms).
	// Если 0, используется TwitterEpoch (1288834974657).
	Epoch        int64
	DatacenterID int64
	MachineID    int64
}

// New создаёт новый Generator.
func New(cfg Config) (*Generator, error) {
	if cfg.DatacenterID < 0 || cfg.DatacenterID > MaxDatacenterID {
		return nil, errors.New("datacenterID must be between 0 and 31")
	}
	if cfg.MachineID < 0 || cfg.MachineID > MaxMachineID {
		return nil, errors.New("machineID must be between 0 and 31")
	}

	epoch := cfg.Epoch
	if epoch == 0 {
		epoch = TwitterEpoch
	}

	return &Generator{
		epoch:         epoch,
		datacenterID:  cfg.DatacenterID,
		machineID:     cfg.MachineID,
		lastTimestamp: -1,
	}, nil
}

// NextID генерирует следующий уникальный 64-битный Snowflake ID.
func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.currentMillis()

	if now < g.lastTimestamp {
		return 0, errors.New("clock moved backwards, refusing to generate ID")
	}

	if now == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & MaxSequence
		if g.sequence == 0 {
			// Исчерпали последовательность — ждём следующей миллисекунды
			now = g.waitNextMillis(g.lastTimestamp)
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = now

	id := ((now - g.epoch) << timestampShift) |
		(g.datacenterID << datacenterShift) |
		(g.machineID << machineShift) |
		g.sequence

	return id, nil
}

// Decompose разбирает Snowflake ID на составные части.
func (g *Generator) Decompose(id int64) DecomposedID {
	ts := (id >> timestampShift) + g.epoch
	return DecomposedID{
		ID:           id,
		Timestamp:    ts,
		Time:         time.UnixMilli(ts),
		DatacenterID: (id >> datacenterShift) & MaxDatacenterID,
		MachineID:    (id >> machineShift) & MaxMachineID,
		Sequence:     id & MaxSequence,
	}
}

// DecomposedID содержит декодированные поля Snowflake ID.
type DecomposedID struct {
	ID           int64
	Timestamp    int64
	Time         time.Time
	DatacenterID int64
	MachineID    int64
	Sequence     int64
}

// currentMillis возвращает текущее время в миллисекундах.
func (g *Generator) currentMillis() int64 {
	return time.Now().UnixMilli()
}

// waitNextMillis ждёт, пока системное время не превысит lastTimestamp.
func (g *Generator) waitNextMillis(last int64) int64 {
	now := g.currentMillis()
	for now <= last {
		now = g.currentMillis()
	}
	return now
}
