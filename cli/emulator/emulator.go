package emulator

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/emulator/start"
)

var Cmd = &cobra.Command{
	Use:              "emulator",
	Short:            "Flow emulator server",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(start.Cmd)
}
