package run

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/rs/zerolog/log"
)

func GenerateExecutionState(priv string) (flow.StateCommitment, error) {
	levelDB := tempLevelDB()

	ledgerStorage, err := ledger.NewTrieStorage(levelDB)
	if err != nil {
		return nil, err
	}

	// TODO return levelDb so it can be dumped/serialized

	return bootstrapLedger(ledgerStorage, priv)
}

func tempLevelDB() *leveldb.LevelDB {
	dir, err := ioutil.TempDir("", "flow-bootstrap-db")
	if err != nil {
		log.Fatal().Err(err).Msg("error creating temp dir")
	}

	kvdbPath := filepath.Join(dir, "kvdb")
	tdbPath := filepath.Join(dir, "tdb")

	db, err := leveldb.NewLevelDB(kvdbPath, tdbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating temp dir")
	}

	return db
}

func bootstrapLedger(ledger storage.Ledger, priv string) (flow.StateCommitment, error) {
	view := state.NewView(state.LedgerGetRegister(ledger, ledger.LatestStateCommitment()))

	err := createRootAccount(view, priv)
	if err != nil {
		return nil, err
	}

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta())
	if err != nil {
		return nil, err
	}

	return newStateCommitment, nil
}

func createRootAccount(view *state.View, priv string) error {
	privateKeyBytes, err := hex.DecodeString(priv)
	if err != nil {
		return fmt.Errorf("cannot hex decode hardcoded key: %w", err)
	}

	privateKey, err := flow.DecodeAccountPrivateKey(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("cannot decode hardcoded private key: %w", err)
	}

	publicKeyBytes, err := flow.EncodeAccountPublicKey(privateKey.PublicKey(1000))
	if err != nil {
		return fmt.Errorf("cannot encode public key of hardcoded private key: %w", err)
	}
	_, err = virtualmachine.CreateAccountInLedger(view, [][]byte{publicKeyBytes})
	if err != nil {
		return fmt.Errorf("error while creating account in ledger: %w", err)
	}

	return nil
}
