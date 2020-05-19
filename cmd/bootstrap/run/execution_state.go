package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

func GenerateAccount0PrivateKey(seed []byte) (flow.AccountPrivateKey, error) {
	priv, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.ECDSASecp256k1,
		HashAlgo:   hash.SHA2_256,
	}, nil
}

func GenerateExecutionState(dbDir string, priv flow.AccountPrivateKey) (flow.StateCommitment, error) {
	metricsCollector := &metrics.NoopCollector{}
	ledgerStorage, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
	defer ledgerStorage.CloseStorage()
	if err != nil {
		return nil, err
	}

	return bootstrapLedger(ledgerStorage, priv)
}

func bootstrapLedger(ledger storage.Ledger, priv flow.AccountPrivateKey) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

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

func createRootAccount(view *delta.View, privateKey flow.AccountPrivateKey) error {
	ledgerAccess := virtualmachine.LedgerDAL{Ledger: view}

	// initialize the account addressing state
	ledgerAccess.SetAddressState(flow.ZeroAddressState)

	// create the root account
	_, err := ledgerAccess.CreateAccountInLedger([]flow.AccountPublicKey{privateKey.PublicKey(1000)})
	if err != nil {
		return fmt.Errorf("error while creating account in ledger: %w", err)
	}

	return nil
}
