package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// clientCmd represents the emulator command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Bamboo Client operations",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("client called")
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}
