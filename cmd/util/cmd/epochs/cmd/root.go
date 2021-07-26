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
	Short: "This tool encapsulates all commands required to interact with Epochs, from recovery to deployment.",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagBootDir, "boot-dir", "b", "", "path to the directory containing the bootstrap files")

	cobra.OnInitialize(initConfig)
	log = zerolog.New(zerolog.NewConsoleWriter())
}

func initConfig() {
	viper.AutomaticEnv()
}
