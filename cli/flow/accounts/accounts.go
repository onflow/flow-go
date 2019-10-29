package accounts

import (
	create2 "github.com/dapperlabs/flow-go/cli/flow/accounts/create"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:              "accounts",
	Short:            "Utilities to manage accounts",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(create2.Cmd)
}
