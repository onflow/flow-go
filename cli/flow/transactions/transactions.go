package transactions

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/flow/transactions/send"
)

var Cmd = &cobra.Command{
	Use:              "transactions",
	Short:            "Utilities to send transactions",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(send.Cmd)
}
