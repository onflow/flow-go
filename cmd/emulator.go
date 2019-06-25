package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// emulatorCmd represents the emulator command
var emulatorCmd = &cobra.Command{
	Use:   "emulator",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("emulator called")
	},
}

func init() {
	rootCmd.AddCommand(emulatorCmd)
}