package cmd

import (
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func genGenesisExecutionState() flow.StateCommitment {
	serviceAccountPriv, err := run.GenerateServiceAccountPrivateKey(generateRandomSeed())
	if err != nil {
		log.Fatal().Err(err).Msg("error generating account 0 private key")
	}

	enc := serviceAccountPriv.PrivateKey.Encode()
	writeJSON(bootstrap.PathServiceAccountPriv, enc)

	dbpath := filepath.Join(flagOutdir, bootstrap.DirnameExecutionState)
	stateCommitment, err := run.GenerateExecutionState(dbpath, serviceAccountPriv)
	if err != nil {
		log.Fatal().Err(err).Msg("error generating execution state")
	}
	log.Info().Msgf("wrote execution state db to directory %v", dbpath)

	writeJSON(bootstrap.PathGenesisCommit, stateCommitment)

	return stateCommitment
}
