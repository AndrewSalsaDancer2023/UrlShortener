package config

import (
	"os"
	"strconv"
)

// Config содержит все параметры микросервиса.
// Значения читаются из переменных окружения с fallback на дефолты.
type Config struct {
	// HTTP
	Port string

	// Snowflake generator
	DatacenterID int64
	MachineID    int64
	Epoch        int64 // Unix ms; 0 = TwitterEpoch
}

func Load() Config {
	return Config{
		Port:         getEnv("PORT", "8080"),
		DatacenterID: getEnvInt64("DATACENTER_ID", 1),
		MachineID:    getEnvInt64("MACHINE_ID", 1),
		Epoch:        getEnvInt64("EPOCH", 0),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
