// Package flow implements the entrypoint for the Flow CLI.
package flow

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli"
	"github.com/dapperlabs/flow-go/cli/flow/accounts"
	"github.com/dapperlabs/flow-go/cli/flow/cadence"
	"github.com/dapperlabs/flow-go/cli/flow/emulator"
	"github.com/dapperlabs/flow-go/cli/flow/initialize"
	"github.com/dapperlabs/flow-go/cli/flow/keys"
	"github.com/dapperlabs/flow-go/cli/flow/transactions"
)

var cmd = &cobra.Command{
	Use:              "flow",
	TraverseChildren: true,
}

func init() {
	cmd.AddCommand(initialize.Cmd)
	cmd.AddCommand(accounts.Cmd)
	cmd.AddCommand(keys.Cmd)
	cmd.AddCommand(emulator.Cmd)
	cmd.AddCommand(cadence.Cmd)
	cmd.AddCommand(transactions.Cmd)
}

func Execute() {
	if err := cmd.Execute(); err != nil {
		cli.Exit(1, err.Error())
	}
}
