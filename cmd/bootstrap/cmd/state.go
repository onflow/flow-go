package cmd

import (
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func genGenesisExecutionState() flow.StateCommitment {
	account0Priv, err := run.GenerateAccount0PrivateKey(generateRandomSeed())
	if err != nil {
		log.Fatal().Err(err).Msg("error generating account 0 private key")
	}

	enc := account0Priv.PrivateKey.Encode()
	writeJSON(bootstrap.FilenameAccount0Priv, enc)

	dbpath := filepath.Join(flagOutdir, bootstrap.DirnameExecutionState)
	stateCommitment, err := run.GenerateExecutionState(dbpath, account0Priv)
	if err != nil {
		log.Fatal().Err(err).Msg("error generating execution state")
	}
	log.Info().Msgf("wrote execution state db to directory %v", dbpath)

	writeJSON(bootstrap.FilenameGenesisCommit, stateCommitment)

	return stateCommitment
}
