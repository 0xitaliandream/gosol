// config/worker.go
package config

import (
	"time"

	"github.com/spf13/viper"
)

type MonitorConfig struct {
	BaseConfig
	RabbitURL             string        `mapstructure:"RABBIT_URL"`
	ExchangeName          string        `mapstructure:"EXCHANGE_NAME"`
	QueueName             string        `mapstructure:"QUEUE_NAME"`
	RoutingKey            string        `mapstructure:"ROUTING_KEY"`
	CheckInterval         time.Duration `mapstructure:"CHECK_INTERVAL"`
	GetSignaturesLimitRpc int           `mapstructure:"GET_SIGNATURES_LIMIT_RPC"`
}

func LoadMonitorConfig() (*MonitorConfig, error) {
	base, err := loadBase()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigName("monitor")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	// Set default values
	v.SetDefault("RABBIT_URL", "amqp://guest:guest@localhost:5672/")
	v.SetDefault("EXCHANGE_NAME", "wallet_exchange")
	v.SetDefault("QUEUE_NAME", "wallet_queue")
	v.SetDefault("ROUTING_KEY", "wallet.check")
	v.SetDefault("CHECK_INTERVAL", "1m")
	v.SetDefault("GET_SIGNATURES_LIMIT_RPC", "1")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config MonitorConfig
	config.BaseConfig = *base

	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Parse check interval string to duration
	if checkInterval := v.GetString("CHECK_INTERVAL"); checkInterval != "" {
		duration, err := time.ParseDuration(checkInterval)
		if err != nil {
			return nil, err
		}
		config.CheckInterval = duration
	}

	return &config, nil
}
