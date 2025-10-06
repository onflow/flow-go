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

// run with `./rollback-executed-height --height 100`
var rootCmd = &cobra.Command{
	Use:   "rollback-executed-height",
	Short: "rollback executed height",
	RunE:  runE,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	common.InitDataDirFlag(rootCmd, &flagDatadir)
	_ = rootCmd.MarkPersistentFlagRequired("datadir")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
