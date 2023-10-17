package version

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/build"
)

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of the utils tool",
	Run:   run,
}

func run(*cobra.Command, []string) {
	version, err := build.Semver()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get version")
	}

	log.Info().Msgf("utils version: %s", version.String())
}
