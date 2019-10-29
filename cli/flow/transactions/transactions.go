package transactions

import (
	send2 "github.com/dapperlabs/flow-go/cli/flow/transactions/send"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:              "transactions",
	Short:            "Utilities to send transactions",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(send2.Cmd)
}
