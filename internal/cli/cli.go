package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/internal/cli/accounts"
	"github.com/dapperlabs/flow-go/internal/cli/bpl"
	"github.com/dapperlabs/flow-go/internal/cli/emulator"
	"github.com/dapperlabs/flow-go/internal/cli/initialize"
	"github.com/dapperlabs/flow-go/internal/cli/keys"
	"github.com/dapperlabs/flow-go/internal/cli/transactions"
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
	cmd.AddCommand(bpl.Cmd)
	cmd.AddCommand(transactions.Cmd)
}

func Execute() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
