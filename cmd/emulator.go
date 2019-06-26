package cmd

import (
	"github.com/spf13/cobra"
)

// emulatorCmd represents the emulator command
var emulatorCmd = &cobra.Command{
	Use:              "emulator",
	Short:            "Bamboo Emulator Server operations",
	TraverseChildren: true,
}

// startCmd represents the emulator start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the Bamboo Emulator Server",
	Run: func(cmd *cobra.Command, args []string) {
		StartServer()
	},
}

func init() {
	rootCmd.AddCommand(emulatorCmd)
	emulatorCmd.AddCommand(startCmd)

	emulatorCmd.PersistentFlags().IntVar(&conf.Port, "port", 0, "port to run emulator server on")
}
