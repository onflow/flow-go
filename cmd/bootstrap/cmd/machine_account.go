package cmd

import (
	"github.com/spf13/cobra"
)

// machineAccountCmd reqpresents the command to generate the `NodeMachineAccountInfo` file
var machineAccountCmd = &cobra.Command{
	Use:   "machine-account",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(machineAccountCmd)
}
