package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-emulator/client"
)

// initCmd represents the emulator command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize new and empty Bamboo project",
	Run: func(cmd *cobra.Command, args []string) {
		client.InitClient(log)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
