package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagChain   string
	flagDatadir string
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
	common.InitDataDirFlag(rootCmd, &flagDatadir)

	rootCmd.PersistentFlags().StringVar(&flagChain, "chain", "", "Chain name, e.g. flow-mainnet, flow-testnet")
	_ = rootCmd.MarkPersistentFlagRequired("chain")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
