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

// run with `./remove-execution-fork `
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
	common.InitDataDirFlag(RootCmd, &flagDatadir)

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
