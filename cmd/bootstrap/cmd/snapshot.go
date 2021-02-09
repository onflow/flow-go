package cmd

import (
	"github.com/spf13/cobra"
)

// snapshotCmd represents the command to download the latest protocol snapshot to bootstrap from
var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Downloads the latest serialized protocol snapshot from an access node to be used to bootstrap from",
	Long:  `Downloads the latest serialized protocol snapshot from an access node to be used to bootstrap from`,
	Run:   snapshot,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)

	// required parameters
	snapshotCmd.Flags().
		StringVar(&flagConfig, "config", "node-config.json", "path to a JSON file containing multiple node configurations (Role, Address, Stake)")
	_ = keygenCmd.MarkFlagRequired("config")
}

func snapshot(cmd *cobra.Command, args []string) {

}
