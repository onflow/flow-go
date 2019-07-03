package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	AppPort    string
	AppEnv     string
	PgAddr     string
	PgUser     string
	PgPassword string
	PgDatabase string
}

// New returns a new Config object
func New() *Config {
	viper.AutomaticEnv()
	return &Config{
		AppPort:    get("port", ""),
		AppEnv:     get("app_env", ""),
		PgAddr:     get("postgres_addr", ""),
		PgUser:     get("postgres_user", ""),
		PgPassword: get("postgres_password", ""),
		PgDatabase: get("postgres_database", ""),
	}
}

func get(key, defaultValue string) string {
	value := viper.GetString(key)
	if value == "" {
		if defaultValue == "" {
			panic(fmt.Sprintf("Config for %v: No value or default value", key))
		}
		return defaultValue
	}
	return value
}
