package testutil

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Create a transaction that will deploy the provided contract
// to the specified address.
func CreateContractDeploymentTransaction(contract string, address flow.Address) flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(contract))
	return flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }`, encoded)),
		Authorizers: []flow.Address{address},
	}
}

func SignTransactionByRoot(tx *flow.TransactionBody, seqNum uint64) error {

	hasher, err := hash.NewHasher(flow.RootAccountPrivateKey.HashAlgo)
	if err != nil {
		return fmt.Errorf("cannot create hasher: %w", err)
	}

	err = tx.SetPayer(flow.RootAddress).
		SetProposalKey(flow.RootAddress, 0, seqNum).
		SignEnvelope(flow.RootAddress, 0, flow.RootAccountPrivateKey.PrivateKey, hasher)

	if err != nil {
		return fmt.Errorf("cannot sign tx: %w", err)
	}

	return nil
}

func RootBootstrappedLedger() (virtualmachine.Ledger, error) {
	ledger := make(virtualmachine.MapLedger)

	return ledger, BootstrapLedgerWithRootAccount(ledger)
}

func BootstrapLedgerWithRootAccount(ledger virtualmachine.Ledger) error {

	ledgerAccess := virtualmachine.LedgerDAL{Ledger: ledger}

	rootAccountPublicKey := flow.RootAccountPrivateKey.PublicKey(virtualmachine.AccountKeyWeightThreshold)

	_, err := ledgerAccess.CreateAccountInLedger([]flow.AccountPublicKey{rootAccountPublicKey})
	if err != nil {
		return err
	}

	return nil
}
