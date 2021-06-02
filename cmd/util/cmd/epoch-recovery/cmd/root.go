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
	Short: `If the epoch setup phase fails (either the DKG, QC voting, or smart contract bug), 
	manual intervention is needed to transition to the next epoch. The manual intervention 
	involves invoking a reset function on the EpochLifecycle smart contract and a spork.`,
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
