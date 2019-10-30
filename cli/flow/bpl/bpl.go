package bpl

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/language/runtime/cmd/execute"
)

var Cmd = &cobra.Command{
	Use:   "bpl",
	Short: "Execute BPL code",
	Run: func(cmd *cobra.Command, args []string) {
		execute.Execute(args)
	},
}
