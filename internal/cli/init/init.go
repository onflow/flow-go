package init

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/client"
)

var Reset bool

func Command(log *logrus.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize new and empty Bamboo project",
		Run: func(cmd *cobra.Command, args []string) {
			client.InitClient(log, Reset)
		},
	}

	cmd.PersistentFlags().BoolVar(&Reset, "reset", false, "reset Bamboo config files")

	return cmd
}
