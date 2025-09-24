package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagDatadir string
)

var rootCmd = &cobra.Command{
	Use:   "reindex",
	Short: "reindex data",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	common.InitDataDirFlag(rootCmd, &flagDatadir)
	_ = rootCmd.MarkPersistentFlagRequired("data-dir")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
