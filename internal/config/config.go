package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	BackendPort                           string
	BackendHost                           string
	ManagementEndpointsWebExposureInclude string
	KafkaTopic                            string
	KafkaPort                             string
	KafkaHost                             string
	ServerPort                            string
}

var (
	AppConfig     Config
	configLoaded  bool
	configLoadMux sync.Mutex
)

func LoadConfig(configPath string) error {
	configLoadMux.Lock()
	defer configLoadMux.Unlock()

	if configLoaded {
		return nil // Config already loaded, no need to read again
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)
	viper.AutomaticEnv() // Read in environment variables that match

	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	AppConfig = Config{
		BackendPort:                           getConfigString("backend.port", "BACKEND_PORT"),
		BackendHost:                           getConfigString("backend.host", "BACKEND_HOST"),
		ManagementEndpointsWebExposureInclude: getConfigString("management.endpoints.web.exposure.include", "MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE"),
		KafkaTopic:                            getConfigString("kafka.topic", "KAFKA_TOPIC"),
		KafkaPort:                             getConfigString("kafka.port", "KAFKA_PORT"),
		KafkaHost:                             getConfigString("kafka.host", "KAFKA_HOST"),
		ServerPort:                            getConfigString("server.port", "SERVER_PORT"),
	}

	configLoaded = true
	return nil
}

func getConfigString(key string, envVar string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return viper.GetString(key)
}

func GetConfig() *Config {
	return &AppConfig
}

func SetBackendPort(port string) {
	configLoadMux.Lock()
	defer configLoadMux.Unlock()

	AppConfig.BackendPort = port
}

func SetKafkaPort(port string) {
	configLoadMux.Lock()
	defer configLoadMux.Unlock()

	AppConfig.KafkaPort = port
}

func ResetConfig() {
	configLoadMux.Lock()
	defer configLoadMux.Unlock()

	configLoaded = false
}
