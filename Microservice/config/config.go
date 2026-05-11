package config

import (
	"flag"
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
	// Epoch        int64 // Unix ms; 0 = TwitterEpoch
}

func Load() Config {

	portFlag := flag.String("port", getEnv("PORT", "50051"), "gRPC server port")
	dcFlag := flag.Int64("dc-id", getEnvInt64("DATACENTER_ID", 0), "Datacenter ID")
	machFlag := flag.Int64("mach-id", getEnvInt64("MACHINE_ID", 0), "Machine ID")

	// 2. Парсим переданные аргументы командной строки.
	// Эта функция перезапишет дефолтные значения, если флаги были переданы вручную.
	flag.Parse()
	/*
		return Config{
			Port:         getEnv("PORT", ":50051"),
			DatacenterID: getEnvInt64("DATACENTER_ID", 0),
			MachineID:    getEnvInt64("MACHINE_ID", 0),
		}
	*/
	return Config{
		Port:         *portFlag,
		DatacenterID: *dcFlag,
		MachineID:    *machFlag,
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
