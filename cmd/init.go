package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// initCmd represents the emulator command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("init called")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}