package emulator

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/config"
	"github.com/dapperlabs/bamboo-node/internal/emulator"
)

var conf emulator.Config

func Command(log *logrus.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "emulator",
		Short:            "Bamboo Emulator Server operations",
		TraverseChildren: true,
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Starts the Bamboo Emulator Server",
		Run: func(cmd *cobra.Command, args []string) {
			emulator.StartServer(log, conf)
		},
	}

	cmd.AddCommand(startCmd)

	cmd.PersistentFlags().IntVarP(&conf.Port, "port", "p", 0, "port to run emulator server on")
	cmd.PersistentFlags().BoolVarP(&conf.Verbose, "verbose", "v", false, "verbose output")

	config.ParseConfig("BAM", &conf, cmd.PersistentFlags())

	return cmd
}
