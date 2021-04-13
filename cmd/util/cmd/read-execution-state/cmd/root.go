package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagExecutionStateDir string
	flagDataDir           string
)

// RootCmd for read-execution-state
var RootCmd = &cobra.Command{
	Use:   "read-execution-state",
	Short: "reads execution state and allows queries against it",
}

// Execute ...
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&flagExecutionStateDir, "execution-state-dir", "", "execution node state dir (where WAL logs are written)")
	_ = RootCmd.MarkPersistentFlagRequired("execution-state-dir")

	RootCmd.PersistentFlags().StringVar(&flagDataDir, "datadir", "", "path to the directory containing the badger dababase")
}
