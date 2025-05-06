package cmd

import (
	"fmt"
	"os"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagDBs common.DBFlags
)

var rootCmd = &cobra.Command{
	Use:   "read-badger",
	Short: "read storage data",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
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
