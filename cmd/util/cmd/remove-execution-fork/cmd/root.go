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
var RootCmd = &cobra.Command{
	Use:   "remove-execution-fork",
	Short: "remove execution fork",
	Run:   run,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&flagDatadir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = RootCmd.MarkPersistentFlagRequired("datadir")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
