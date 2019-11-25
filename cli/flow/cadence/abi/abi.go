package abi

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "abi",
	Short: "Generates JSON ABI from given Cadence file",
	Run: func(cmd *cobra.Command, args []string) {

	},
}
