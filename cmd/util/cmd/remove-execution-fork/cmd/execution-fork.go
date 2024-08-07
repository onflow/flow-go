package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDatadir).
		Msg("flags")

	db, err := common.InitStoragePebble(flagDatadir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not open db")
	}
	defer db.Close()

	err = operation.RemoveExecutionForkEvidence()(db)

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
