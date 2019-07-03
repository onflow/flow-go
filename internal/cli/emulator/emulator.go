package emulator

import (
	"github.com/psiemens/sconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

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

	err := sconfig.New(&conf).
		FromEnvironment("BAM").
		BindFlags(cmd.PersistentFlags()).
		Parse()
	if err != nil {
		log.Fatal(err)
	}

	return cmd
}
