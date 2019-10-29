package transactions

import (
	"github.com/dapperlabs/flow-go/cli/transactions/send"
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
