package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

func GenerateAccount0PrivateKey(seed []byte) (flow.AccountPrivateKey, error) {
	priv, err := crypto.GeneratePrivateKey(crypto.EcdsaSecp256k1, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.EcdsaSecp256k1,
		HashAlgo:   hash.SHA2_256,
	}, nil
}

func GenerateExecutionState(dbDir string, priv flow.AccountPrivateKey) (flow.StateCommitment, error) {
	ledgerStorage, err := ledger.NewTrieStorage(dbDir)
	defer ledgerStorage.CloseStorage()
	if err != nil {
		return nil, err
	}

	return bootstrapLedger(ledgerStorage, priv)
}

func bootstrapLedger(ledger storage.Ledger, priv flow.AccountPrivateKey) (flow.StateCommitment, error) {
	view := state.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	err := createRootAccount(view, priv)
	if err != nil {
		return nil, err
	}

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta(), ledger.EmptyStateCommitment())
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
