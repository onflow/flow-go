package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-emulator/client"
)

var Reset bool

// initCmd represents the emulator command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize new and empty Bamboo project",
	Run: func(cmd *cobra.Command, args []string) {
		client.InitClient(log, Reset)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.PersistentFlags().BoolVar(&Reset, "reset", false, "reset Bamboo config files")
}
