package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	BackendPort   string
	BackendHost   string
	KafkaTopic    string
	KafkaPort     string
	KafkaHost     string
	BFFServerPort string
}

func LoadConfig() (*Config, error) {

	var config Config

	viper.SetConfigName("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv() // Read in environment variables that match

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	config = Config{
		BackendPort:   getConfigString("backend.port", "DOMAIN_SERVER_PORT"),
		BackendHost:   getConfigString("backend.host", "DOMAIN_SERVER_HOST"),
		KafkaTopic:    getConfigString("kafka.topic", "KAFKA_TOPIC"),
		KafkaPort:     getConfigString("kafka.port", "KAFKA_PORT"),
		KafkaHost:     getConfigString("kafka.host", "KAFKA_HOST"),
		BFFServerPort: getConfigString("bff_server.port", "SERVER_PORT"),
	}

	return &config, nil
}

func getConfigString(key string, envVar string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return viper.GetString(key)
}
