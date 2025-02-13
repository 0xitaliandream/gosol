// config/api.go
package config

import "github.com/spf13/viper"

type APIConfig struct {
	BaseConfig
	API_PORT string `mapstructure:"API_PORT"`
}

func LoadAPIConfig() (*APIConfig, error) {
	base, err := LoadBase()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigName("api")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	v.SetDefault("API_PORT", "8080")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// Ignoriamo l'errore se il file non esiste
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config APIConfig
	config.BaseConfig = *base

	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
