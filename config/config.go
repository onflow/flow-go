package config

import (
	"bytes"
	"embed"
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const configFileName = "config.yml"

var (
	conf = viper.New()

	//go:embed config.yml
	configFile embed.FS
)

// Initialize initializes the flow configuration. All default values for the Flow
// configuration are stored in the config.yml file. These values can be overriden
// by node operators by setting the corresponding cli flag. Initialize should be called
// before any pflags are parsed, this will allow the configuration to initialize with defaults
// from config.yml.
func Initialize() error {
	f, err := configFile.Open(configFileName)
	if err != nil {
		return fmt.Errorf("failed to open config.yml: %w", err)
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(f)
	if err != nil {
		return fmt.Errorf("failed to read config file into bytes buffer: %w", err)
	}

	conf.SetConfigType("yaml")

	if err = conf.ReadConfig(buf); err != nil {
		return fmt.Errorf("failed to initialize flow config failed to read in config file: %w", err)
	}

	return nil
}

// BindPFlags binds the configuration to the cli pflag set. This should be called
// after all pflags have been parsed.
func BindPFlags() error {
	if err := conf.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("failed to bind pflags: %w", err)
	}
	return nil
}
