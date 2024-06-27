package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	BackendAPI                            string
	ManagementEndpointsWebExposureInclude string
	KafkaBootstrapServers                 string
	KafkaTopic                            string
	ServerPort                            string
}

var AppConfig Config

func LoadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	AppConfig = Config{
		BackendAPI:                            viper.GetString("backend.api"),
		ManagementEndpointsWebExposureInclude: viper.GetString("management.endpoints.web.exposure.include"),
		KafkaBootstrapServers:                 viper.GetString("kafka.bootstrap-servers"),
		KafkaTopic:                            viper.GetString("kafka.topic"),
		ServerPort:                            viper.GetString("server.port"),
	}

	return nil
}

func GetConfig() *Config {
	return &AppConfig
}
