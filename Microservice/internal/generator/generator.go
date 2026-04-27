package generator

import (
	"errors"
	"sync"
	"time"
)

const (
	TwitterEpoch int64 = 1288834974657

	timestampBits  = 41
	datacenterBits = 5
	machineBits    = 5
	sequenceBits   = 12

	MaxDatacenterID = (1 << datacenterBits) - 1 // 31
	MaxMachineID    = (1 << machineBits) - 1    // 31
	MaxSequence     = (1 << sequenceBits) - 1   // 4095

	machineShift    = sequenceBits
	datacenterShift = sequenceBits + machineBits
	timestampShift  = sequenceBits + machineBits + datacenterBits
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
}

type Config struct {
	Epoch        int64
	DatacenterID int64
	MachineID    int64
}

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

func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()

	if now < g.lastTimestamp {
		return 0, errors.New("clock moved backwards, refusing to generate ID")
	}
	if now == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & MaxSequence
		if g.sequence == 0 {
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

func (g *Generator) waitNextMillis(last int64) int64 {
	now := time.Now().UnixMilli()
	for now <= last {
		now = time.Now().UnixMilli()
	}
	return now
}
