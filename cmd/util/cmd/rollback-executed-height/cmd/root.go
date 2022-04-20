package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagDatadir string
)

// run with `./remove-execution-fork --datadir /var/flow/data/protocol`
var rootCmd = &cobra.Command{
	Use:   "remove-execution-result",
	Short: "remove execution result",
	Run:   run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagDatadir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = rootCmd.MarkPersistentFlagRequired("datadir")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
