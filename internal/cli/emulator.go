package cli

import (
	"github.com/spf13/cobra"

	"github.com/dapperlabs/bamboo-node/internal/emulator/server"
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
		server.StartServer(log, conf)
	},
}

func init() {
	rootCmd.AddCommand(emulatorCmd)
	emulatorCmd.AddCommand(startCmd)

	emulatorCmd.PersistentFlags().IntVarP(&conf.Port, "port", "p", 0, "port to run emulator server on")
	emulatorCmd.PersistentFlags().BoolVarP(&conf.Verbose, "verbose", "v", false, "verbose output")
}
