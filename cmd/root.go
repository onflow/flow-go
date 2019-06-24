package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-emulator/config"
	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/nodes/access"
	"github.com/dapperlabs/bamboo-emulator/nodes/security"
	"github.com/dapperlabs/bamboo-emulator/server"
)

type Config struct {
	Port               int           `default:"5000" flag:"port"`
	CollectionInterval time.Duration `default:"1s"`
	BlockInterval      time.Duration `default:"5s"`
}

var (
	conf Config
	log  *logrus.Logger
)

var rootCmd = &cobra.Command{
	Use: "bamboo-emulator",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

func startServer() {
	log.WithField("port", conf.Port).Info("Starting emulator server...")

	collections := make(chan *data.Collection, 16)

	state, err := data.NewWorldState(log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize world state")
	}


	accessNode := access.NewNode(
		&access.Config{
			CollectionInterval: conf.CollectionInterval,
		},
		state,
		collections,
		log,
	)
	securityNode := security.NewNode(
		&security.Config{
			BlockInterval: conf.BlockInterval,
		},
		state,
		collections,
		log,
	)

	emulatorServer := server.NewServer(accessNode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go accessNode.Start(ctx)
	go securityNode.Start(ctx)

	emulatorServer.Start(conf.Port)
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
