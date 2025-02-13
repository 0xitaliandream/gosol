// config/base.go
package config

import (
	"github.com/spf13/viper"
)

type BaseConfig struct {
	DBHosts      []string `mapstructure:"DB_HOSTS"`
	DBName       string   `mapstructure:"DB_NAME"`
	DBUser       string   `mapstructure:"DB_USER"`
	DBPassword   string   `mapstructure:"DB_PASSWORD"`
	DBDebug      bool     `mapstructure:"DB_DEBUG"`
	SolanaRPCURL string   `mapstructure:"SOLANA_RPC_URL"`
}

func loadBase() (*BaseConfig, error) {
	v := viper.New()

	// Impostazioni di base comuni
	v.SetConfigName("base")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	// Default values
	v.SetDefault("DB_HOSTS", []string{"clickhouse:9000"})
	v.SetDefault("DB_NAME", "default")
	v.SetDefault("DB_USER", "default")
	v.SetDefault("DB_PASSWORD", "clickhouse")
	v.SetDefault("DB_DEBUG", false)
	v.SetDefault("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var config BaseConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
