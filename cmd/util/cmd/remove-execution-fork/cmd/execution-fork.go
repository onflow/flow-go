package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

var Cmd = &cobra.Command{
	Use:   "remove-execution-fork",
	Short: "Remove execution fork for conflicting results. Useful for resuming the sealing.",
	Run:   run,
}

func init() {
	rootCmd.AddCommand(Cmd)
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDatadir).
		Msg("flags")

	db := common.InitStorage(flagDatadir)
	defer db.Close()

	err := db.Update(operation.RemoveExecutionForkEvidence())

	// for testing purpose
	// expectedSeals := unittest.IncorporatedResultSeal.Fixtures(2)
	// err := db.Update(operation.InsertExecutionForkEvidence(expectedSeals))

	if err == storage.ErrNotFound {
		log.Info().Msg("no execution fork was found, exit")
		return
	}

	if err != nil {
		log.Fatal().Err(err).Msg("could not remove execution fork")
		return
	}

	log.Info().Msg("execution fork removed")
}
