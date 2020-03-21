package cmd

import (
	"path/filepath"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/rs/zerolog/log"
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
	writeJSON("account-0.priv.json", enc)

	db := createLevelDB(filepath.Join(outdir, "execution-state"))
	defer db.SafeClose()
	stateCommitment, err := run.GenerateExecutionState(db, account0Priv)
	if err != nil {
		log.Fatal().Err(err).Msg("error generating execution state")
	}

	return stateCommitment
}
