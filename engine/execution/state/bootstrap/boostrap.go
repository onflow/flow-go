package bootstrap

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// BootstrapLedger adds the above root account to the ledger
func BootstrapLedger(ledger storage.Ledger) (flow.StateCommitment, error) {
	view := state.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	BootstrapView(view)

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta(), ledger.EmptyStateCommitment())
	if err != nil {
		return nil, err
	}

	return newStateCommitment, nil
}

func BootstrapView(view *state.View) {
	privateKeyBytes, err := hex.DecodeString(flow.RootAccountPrivateKeyHex)
	if err != nil {
		panic("Cannot hex decode hardcoded key!")
	}

	privateKey, err := flow.DecodeAccountPrivateKey(privateKeyBytes)
	if err != nil {
		panic("Cannot decode hardcoded private key!")
	}

	publicKeyBytes, err := flow.EncodeAccountPublicKey(privateKey.PublicKey(1000))
	if err != nil {
		panic("Cannot encode public key of hardcoded private key!")
	}
	_, err = virtualmachine.CreateAccountInLedger(view, [][]byte{publicKeyBytes})
	if err != nil {
		panic(fmt.Sprintf("error while creating account in ledger: %s ", err))
	}
}
