package cmd

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-emulator/config"
)

var (
	conf Config
	log  *logrus.Logger
)

var rootCmd = &cobra.Command{
	Use: "bamboo-emulator",
	Run: func(cmd *cobra.Command, args []string) {
		StartServer()
	},
}

func init() {
	rootCmd.PersistentFlags().IntVar(&conf.Port, "port", 0, "port to run emulator server on")

	initConfig()
	initLogger()
}

func initConfig() {
	config.ParseConfig("BE", &conf, rootCmd.PersistentFlags())
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
