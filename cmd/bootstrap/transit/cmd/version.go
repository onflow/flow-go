package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/build"
)

// versionCmd prints the current versioning information about the Transit CLI
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Transit version information",
	Long:  `Print Transit version information`,
	Run:   version,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// version prints the version information about Transit
func version(cmd *cobra.Command, args []string) {

	// Print version/commit strings if they are known
	if build.IsDefined(semver) {
		fmt.Printf("Transit script Version: %s\n", semver)
	}
	if build.IsDefined(commit) {
		fmt.Printf("Transit script Commit: %s\n", commit)
	}
	// If no version info is known print a message to indicate this.
	if !build.IsDefined(semver) && !build.IsDefined(commit) {
		fmt.Printf("Transit script version information unknown\n")
	}
}
