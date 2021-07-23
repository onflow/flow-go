package utils

import (
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Logger zerolog.Logger

func init() {
	Logger = zerolog.New(zerolog.NewConsoleWriter())
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
