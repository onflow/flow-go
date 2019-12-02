package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Default value for build-time-injected version strings.
const unknown = "undefined"

var (
	// The Git commit hash, injected at build time
	commit = unknown
	// The semver version string, injected at build time
	version = unknown
)

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "View version and commit information",
	Run: func(cmd *cobra.Command, args []string) {
		// Print version/commit strings if they are known
		if isKnown(version) {
			fmt.Printf("Version: %s\n", version)
		}
		if isKnown(commit) {
			fmt.Printf("Commit: %s\n", commit)
		}
		// If no version info is known print a message to indicate this.
		if !isKnown(version) && !isKnown(commit) {
			fmt.Printf("Version information unknown!\n")
		}
	},
}

func isKnown(version string) bool {
	return version != unknown && len(version) > 0
}
