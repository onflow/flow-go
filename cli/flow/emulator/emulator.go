package emulator

import (
	start2 "github.com/dapperlabs/flow-go/cli/flow/emulator/start"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:              "emulator",
	Short:            "Flow emulator server",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(start2.Cmd)
}
