package config

import (
	"log"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	ShortURLPath         string `yaml:"short_url_path"`
	OriginalURLPath      string `yaml:"original_url_path"`
	OriginalURLPathShort string `yaml:"original_url_path_short"`

	GatewayURL string `yaml:"gateway_url"`
	EngineURL  string `yaml:"engine_url"`
	CacheURL   string `yaml:"cache_url"`

	ProtocolKey     string `yaml:"protocol_key"`
	DoubleSeparator string `yaml:"double_separator"`
	LongURLKey      string `yaml:"post_body_key"`
}

var config Config

func init() {
	configData, err := os.ReadFile("../../config/gateway.yaml")
	if err != nil {
		log.Fatalf("Error reading config: %s \n", err.Error())
	}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		log.Fatalf("Error parsing config: %s \n", err.Error())
	}
}

func GetConfig() *Config {
	return &config
}
