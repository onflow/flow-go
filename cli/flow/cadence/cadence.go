package cadence

import (
	"github.com/dapperlabs/flow-go/cli/flow/cadence/languageserver"
	"github.com/dapperlabs/flow-go/language/runtime/cmd/execute"
	"github.com/spf13/cobra"
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
