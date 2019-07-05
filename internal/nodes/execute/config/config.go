package config

import "github.com/psiemens/sconfig"

// Config holds the application configuration for an execution node.
type Config struct {
	Port int `default:"5000"`
}

// New returns a new Config object.
func New() *Config {
	var conf Config

	err := sconfig.New(&conf).
		FromEnvironment("BAM").
		Parse()

	if err != nil {
		panic(err.Error())
	}

	return &conf
}
