package cmd

import (
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
	writeYaml("account-0.priv.yml", enc)

	stateCommitment, err := run.GenerateExecutionState(account0Priv)
	if err != nil {
		log.Fatal().Err(err).Msg("error generating execution state")
	}

	// TODO write genesis execution state to file

	return stateCommitment
}
