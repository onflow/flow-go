package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagDBs common.DBFlags
)

var rootCmd = &cobra.Command{
	Use:   "read-protocol-state",
	Short: "read storage data",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	flagDBs = common.InitWithDBFlags(rootCmd)

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
