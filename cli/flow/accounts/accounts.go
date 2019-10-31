package accounts

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/flow/accounts/create"
)

var Cmd = &cobra.Command{
	Use:              "accounts",
	Short:            "Utilities to manage accounts",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(create.Cmd)
}
