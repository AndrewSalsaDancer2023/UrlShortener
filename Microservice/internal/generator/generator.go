package generator

import (
	"errors"
	"sync"
	"time"
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
}

type Config struct {
	//	Epoch        int64
	DatacenterID int64
	MachineID    int64
}

func New(cfg Config) (*Generator, error) {
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
	}, nil
}

func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().Unix()

	if now < g.lastTimestamp {
		return 0, errors.New("clock moved backwards, refusing to generate ID")
	}
	if now == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & MaxSequence
		if g.sequence == 0 {
			now = g.waitNextSecond(g.lastTimestamp)
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

func (g *Generator) waitNextSecond(last int64) int64 {
	now := time.Now().Unix()
	for now <= last {
		//		time.Sleep(10 * time.Millisecond)
		now = time.Now().Unix()
	}

	return now
}
