package run

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	"github.com/rs/zerolog/log"
)

func GenerateAccount0PrivateKey(seed []byte) (flow.AccountPrivateKey, error) {
	priv, err := crypto.GeneratePrivateKey(crypto.ECDSA_SECp256k1, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.ECDSA_SECp256k1,
		HashAlgo:   crypto.SHA2_256,
	}, nil
}

func GenerateExecutionState(priv flow.AccountPrivateKey) (flow.StateCommitment, error) {
	levelDB := tempLevelDB()

	ledgerStorage, err := ledger.NewTrieStorage(levelDB)
	if err != nil {
		return nil, err
	}

	// TODO return levelDb so it can be dumped/serialized

	return bootstrapLedger(ledgerStorage, priv)
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}

func tempLevelDB() *leveldb.LevelDB {
	dir, err := tempDBDir()
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

func bootstrapLedger(ledger storage.Ledger, priv flow.AccountPrivateKey) (flow.StateCommitment, error) {
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

func createRootAccount(view *state.View, privateKey flow.AccountPrivateKey) error {
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
