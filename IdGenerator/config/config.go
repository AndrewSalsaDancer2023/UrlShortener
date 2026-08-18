package config

import (
	"flag"
	"urlshortener/internal/dbstorage/config"
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

	portFlag := flag.String("port", config.GetEnv("PORT", "50051"), "gRPC server port")
	dcFlag := flag.Int64("dc-id", config.GetEnvInt64("DATACENTER_ID", 0), "Datacenter ID")
	machFlag := flag.Int64("mach-id", config.GetEnvInt64("MACHINE_ID", 0), "Machine ID")

	// 2. Парсим переданные аргументы командной строки.
	// Эта функция перезапишет дефолтные значения, если флаги были переданы вручную.
	flag.Parse()

	return Config{
		Port:         *portFlag,
		DatacenterID: *dcFlag,
		MachineID:    *machFlag,
	}
}
