package cli

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/config"
	"github.com/dapperlabs/bamboo-node/internal/emulator/server"
)

var (
	conf server.Config
	log  *logrus.Logger
)

var rootCmd = &cobra.Command{
	Use:              "bamboo",
	TraverseChildren: true,
}

func init() {
	initConfig()
	initLogger()
}

func initConfig() {
	config.ParseConfig("BAMBOO", &conf, emulatorCmd.PersistentFlags())
}

func initLogger() {
	log = logrus.New()
	log.Formatter = new(logrus.TextFormatter)
	log.Out = os.Stdout
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
