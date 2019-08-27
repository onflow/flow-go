package transactions

import (
	"github.com/dapperlabs/bamboo-node/internal/cli/transactions/send"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:              "transactions",
	Short:            "Utilities to send transactions",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(send.Cmd)
}
