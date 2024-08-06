package config

import (
	"fmt"
	"os"
)

type Config struct {
	BackendPort   string
	BackendHost   string
	KafkaTopic    string
	KafkaPort     string
	KafkaHost     string
	KafkaAPIPort  string
	BFFServerPort string
}

func LoadConfig() (*Config, error) {
	config := &Config{
		BackendPort:   getEnvOrDefault("DOMAIN_SERVER_PORT", "9000"),
		BackendHost:   getEnvOrDefault("DOMAIN_SERVER_HOST", "order-api-mock"),
		KafkaTopic:    getEnvOrDefault("KAFKA_TOPIC", "product-queries"),
		KafkaPort:     getEnvOrDefault("KAFKA_PORT", "9093"),
		KafkaHost:     getEnvOrDefault("KAFKA_HOST", "specmatic-kafka"),
		KafkaAPIPort:  getEnvOrDefault("KAFKA_API_PORT", "9094"),
		BFFServerPort: getEnvOrDefault("SERVER_PORT", "8080"),
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		fmt.Printf("%s received via env var: %s\n", key, value)
		return value
	}
	fmt.Printf("%s using default value: %s\n", key, defaultValue)
	return defaultValue
}
