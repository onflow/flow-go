package cmd

import "time"

type Config struct {
	Port               int           `default:"5000" flag:"port"`
	CollectionInterval time.Duration `default:"2s"`
	BlockInterval      time.Duration `default:"5s"`
}
