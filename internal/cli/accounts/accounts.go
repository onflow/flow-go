package accounts

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/accounts/create"
)

var Cmd = &cobra.Command{
	Use:              "accounts",
	Short:            "Utilities to manage accounts",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(create.Cmd)
}
