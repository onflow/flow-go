package accounts

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:              "accounts",
	Short:            "Manage user accounts",
	TraverseChildren: true,
}
