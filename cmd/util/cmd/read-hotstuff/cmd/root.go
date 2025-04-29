package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagDatadir   string
	flagPebbleDir string
	flagChain     string
)

var rootCmd = &cobra.Command{
	Use:   "read-hotstuff",
	Short: "read hotstuff liveness/safety data",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagDatadir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = rootCmd.MarkPersistentFlagRequired("datadir")

	rootCmd.PersistentFlags().StringVar(&flagPebbleDir, "pebble-dir", "/var/flow/data/protocol-pebble", "directory to the pebble dababase")

	rootCmd.Flags().StringVar(&flagChain, "chain", "", "Chain name, e.g. flow-mainnet, flow-testnet")
	_ = rootCmd.MarkFlagRequired("chain")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
