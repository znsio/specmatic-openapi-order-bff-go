package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	BackendAPI                            string
	BackendPort                           string
	BackendHost                           string
	ManagementEndpointsWebExposureInclude string
	KafkaBootstrapServers                 string
	KafkaTopic                            string
	KafkaPort                             string
	KafkaHost                             string
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
		BackendPort:                           viper.GetString("backend.port"),
		BackendHost:                           viper.GetString("backend.host"),
		ManagementEndpointsWebExposureInclude: viper.GetString("management.endpoints.web.exposure.include"),
		KafkaBootstrapServers:                 viper.GetString("kafka.bootstrap-servers"),
		KafkaTopic:                            viper.GetString("kafka.topic"),
		KafkaPort:                             viper.GetString("kafka.port"),
		KafkaHost:                             viper.GetString("kafka.host"),
		ServerPort:                            viper.GetString("server.port"),
	}

	return nil
}

func GetConfig() *Config {
	return &AppConfig
}
