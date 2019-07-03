package cli

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/client"
	"github.com/dapperlabs/bamboo-node/internal/cli/emulator"
	initCmd "github.com/dapperlabs/bamboo-node/internal/cli/init"
)

var log *logrus.Logger

var cmd = &cobra.Command{
	Use:              "bamboo",
	TraverseChildren: true,
}

func init() {
	initLogger()

	cmd.AddCommand(initCmd.Command(log))
	cmd.AddCommand(client.Command(log))
	cmd.AddCommand(emulator.Command(log))
}

func initLogger() {
	log = logrus.New()
	log.Formatter = new(logrus.TextFormatter)
	log.Out = os.Stdout
}

func Execute() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
