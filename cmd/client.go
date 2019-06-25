package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// clientCmd represents the emulator command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client called")
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}