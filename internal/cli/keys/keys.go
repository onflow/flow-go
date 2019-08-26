package keys

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/cli/keys/generate"
)

var Cmd = &cobra.Command{
	Use:              "keys",
	Short:            "Utilities to manage cryptographic keys",
	TraverseChildren: true,
}

func init() {
	Cmd.AddCommand(generate.Cmd)
}
