package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	log zerolog.Logger
)

var rootCmd = &cobra.Command{
	Use:   "epochs",
	Short: "This too encapsulates all commands required to interact with Epochs, from recovery to deployment.",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	log = zerolog.New(zerolog.NewConsoleWriter())
}

func initConfig() {
	viper.AutomaticEnv()
}
