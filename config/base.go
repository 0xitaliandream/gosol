// config/base.go
package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type BaseConfig struct {
	// ClickHouse Configuration
	CHHosts    []string `mapstructure:"CH_HOSTS"`
	CHDb       string   `mapstructure:"CH_NAME"`
	CHUser     string   `mapstructure:"CH_USER"`
	CHPassword string   `mapstructure:"CH_PASSWORD"`
	CHDebug    bool     `mapstructure:"CH_DEBUG"`

	// MySQL Configuration
	MySQLHost     string `mapstructure:"MYSQL_HOST"`
	MySQLPort     int    `mapstructure:"MYSQL_PORT"`
	MySQLDb       string `mapstructure:"MYSQL_NAME"`
	MySQLUser     string `mapstructure:"MYSQL_USER"`
	MySQLPassword string `mapstructure:"MYSQL_PASSWORD"`
	MySQLDebug    bool   `mapstructure:"MYSQL_DEBUG"`

	// Solana Configuration
	SolanaRPCURL string `mapstructure:"SOLANA_RPC_URL"`
}

func LoadBase() (*BaseConfig, error) {
	v := viper.New()

	// Impostazioni di base comuni
	v.SetConfigName("base")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	// ClickHouse defaults
	v.SetDefault("CH_HOSTS", []string{"clickhouse:9000"})
	v.SetDefault("CH_NAME", "default")
	v.SetDefault("CH_USER", "default")
	v.SetDefault("CH_PASSWORD", "clickhouse")
	v.SetDefault("CH_DEBUG", false)

	// MySQL defaults
	v.SetDefault("MYSQL_HOST", "localhost")
	v.SetDefault("MYSQL_PORT", 3306)
	v.SetDefault("MYSQL_NAME", "solanadb")
	v.SetDefault("MYSQL_USER", "root")
	v.SetDefault("MYSQL_PASSWORD", "root")
	v.SetDefault("MYSQL_DEBUG", false)

	// Solana defaults
	v.SetDefault("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config BaseConfig
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}

	return &config, nil
}
