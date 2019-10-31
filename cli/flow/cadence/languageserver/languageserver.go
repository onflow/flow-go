package languageserver

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/language/tools/language-server/server"
)

var Cmd = &cobra.Command{
	Use:   "language-server",
	Short: "Start the Cadence language server",
	Run: func(cmd *cobra.Command, args []string) {
		server.NewServer().Start()
	},
}
