package emulator

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/emulator/start"
)

var Cmd = &cobra.Command{
	Use:              "emulator",
	Short:            "Bamboo emulator server",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(start.Cmd)
}
