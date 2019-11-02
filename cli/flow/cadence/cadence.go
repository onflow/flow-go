package cadence

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cli/flow/cadence/languageserver"
	"github.com/dapperlabs/flow-go/language/runtime/cmd/execute"
)

var Cmd = &cobra.Command{
	Use:   "cadence",
	Short: "Execute Cadence code",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			execute.Execute(args)
		} else {
			execute.RunREPL()
		}
	},
}

func init() {
	Cmd.AddCommand(languageserver.Cmd)
}
