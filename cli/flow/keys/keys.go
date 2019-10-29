package keys

import (
	generate2 "github.com/dapperlabs/flow-go/cli/flow/keys/generate"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:              "keys",
	Short:            "Utilities to manage cryptographic keys",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(generate2.Cmd)
}
