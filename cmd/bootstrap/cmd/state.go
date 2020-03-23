package cmd

import (
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
)

func genGenesisExecutionState() flow.StateCommitment {
	account0Priv, err := run.GenerateAccount0PrivateKey(generateRandomSeed())
	if err != nil {
		log.Fatal().Err(err).Msg("error generating account 0 private key")
	}

	enc, err := account0Priv.PrivateKey.Encode()
	if err != nil {
		log.Fatal().Err(err).Msg("error encoding account 0 private key")
	}
	writeJSON(filenameAccount0Priv, enc)

	dbpath := filepath.Join(flagOutdir, dirnameExecutionState)
	db := createLevelDB(dbpath)
	defer db.SafeClose()
	stateCommitment, err := run.GenerateExecutionState(db, account0Priv)
	if err != nil {
		log.Fatal().Err(err).Msg("error generating execution state")
	}
	log.Info().Msgf("wrote execution state db to directory %v", dbpath)

	return stateCommitment
}
