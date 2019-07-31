package config

import "github.com/psiemens/sconfig"

// Config holds the application configuration for a collection node.
type Config struct {
	Port             int    `default:"5000"`
	PostgresAddr     string `default:"127.0.0.1:5432"`
	PostgresUser     string `default:"postgres"`
	PostgresPassword string `default:""`
	PostgresDatabase string `default:"bam_collection"`
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
