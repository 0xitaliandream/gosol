// config/txdetails.go
package config

import (
	"time"

	"github.com/spf13/viper"
)

type TxDetailsConfig struct {
	BaseConfig
	RabbitURL     string        `mapstructure:"RABBIT_URL"`
	ExchangeName  string        `mapstructure:"EXCHANGE_NAME"`
	QueueName     string        `mapstructure:"QUEUE_NAME"`
	RoutingKey    string        `mapstructure:"ROUTING_KEY"`
	CheckInterval time.Duration `mapstructure:"CHECK_INTERVAL"`
}

func LoadTxDetailsConfig() (*TxDetailsConfig, error) {
	base, err := LoadBase()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigName("txdetails")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	// Set default values
	v.SetDefault("RABBIT_URL", "amqp://guest:guest@localhost:5672/")
	v.SetDefault("EXCHANGE_NAME", "txdetails_exchange")
	v.SetDefault("QUEUE_NAME", "txdetails_queue")
	v.SetDefault("ROUTING_KEY", "transaction.details")
	v.SetDefault("CHECK_INTERVAL", "1m")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config TxDetailsConfig
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
