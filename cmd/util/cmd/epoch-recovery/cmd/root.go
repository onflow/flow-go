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
	Use: "epoch-recovery",
	Short: "If the epoch setup phase fails (either the DKG, QC voting, or smart contract bug)," +
		"manual intervention is needed to transition to the next epoch. This tool encapsulates the commands" +
		"needed for such intervention.",
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
