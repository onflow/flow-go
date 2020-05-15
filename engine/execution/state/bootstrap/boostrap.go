package bootstrap

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// BootstrapLedger adds the above root account to the ledger and initializes execution node-only data
func BootstrapLedger(ledger storage.Ledger) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	BootstrapView(view)

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta(), ledger.EmptyStateCommitment())
	if err != nil {
		return nil, err
	}

	return newStateCommitment, nil
}

func BootstrapExecutionDatabase(db *badger.DB, commit flow.StateCommitment, genesis *flow.Header) error {
	err := operation.RetryOnConflict(db.Update, func(txn *badger.Txn) error {

		err := operation.InsertExecutedBlock(genesis.ID())(txn)
		if err != nil {
			return fmt.Errorf("could not index initial genesis execution block: %w", err)
		}

		err = operation.IndexStateCommitment(flow.ZeroID, commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index void state commitment: %w", err)
		}

		err = operation.IndexStateCommitment(genesis.ID(), commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index genesis state commitment: %w", err)
		}

		views := make([]*delta.Snapshot, 0)
		err = operation.InsertExecutionStateInteractions(genesis.ID(), views)(txn)
		if err != nil {
			return fmt.Errorf("could not bootstrap execution state interactions: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func BootstrapView(view *delta.View) {
	l := virtualmachine.NewLedgerDAL(view)

	fungibleToken := deployFungibleToken(l)
	flowToken := deployFlowToken(l, fungibleToken)
	feeContract := deployFeeContract(l, fungibleToken, flowToken)

	createRootAccount(l, fungibleToken, flowToken, feeContract)
}

func deployFungibleToken(l virtualmachine.LedgerDAL) flow.Address {
	addr, err := l.CreateAccount(nil, fungibleTokenContract())
	if err != nil {
		panic(fmt.Sprintf("failed to deploy fungible token contract: %s", err))
	}

	return addr
}

func deployFlowToken(l virtualmachine.LedgerDAL, fungibleToken flow.Address) flow.Address {
	addr, err := l.CreateAccount(nil, flowTokenContract(fungibleToken))
	if err != nil {
		panic(fmt.Sprintf("failed to deploy flow token contract: %s", err))
	}

	return addr
}

func deployFeeContract(l virtualmachine.LedgerDAL, fungibleToken, flowToken flow.Address) flow.Address {
	addr, err := l.CreateAccount(nil, feeContract(fungibleToken, flowToken))
	if err != nil {
		panic(fmt.Sprintf("failed to deploy fee contract: %s", err))
	}

	return addr
}

func createRootAccount(
	l virtualmachine.LedgerDAL,
	fungibleToken, flowToken, feeContract flow.Address,
) {
	// TODO: remove magic constant
	accountKey := flow.RootAccountPrivateKey.PublicKey(1000)

	err := l.CreateAccountWithAddress(
		flow.ZeroAddress,
		[]flow.AccountPublicKey{accountKey},
		serviceAccountContract(fungibleToken, flowToken, feeContract),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create root account: %s", err))
	}
}

func fungibleTokenContract() []byte {
	code, err := Asset("contracts/FungibleToken.cdc")
	if err != nil {
		panic(err)
	}

	return code
}

func flowTokenContract(fungibleToken flow.Address) []byte {
	tpl, err := AssetString("contracts/FlowToken.cdc")
	if err != nil {
		panic(err)
	}

	code := fmt.Sprintf(tpl, fungibleToken.Hex())

	return []byte(code)
}

func feeContract(fungibleToken, flowToken flow.Address) []byte {
	tpl, err := AssetString("contracts/FeeContract.cdc")
	if err != nil {
		panic(err)
	}

	code := fmt.Sprintf(tpl, fungibleToken.Hex(), flowToken.Hex())

	return []byte(code)
}

func serviceAccountContract(fungibleToken, flowToken, feeContract flow.Address) []byte {
	tpl, err := AssetString("contracts/ServiceAccount.cdc")
	if err != nil {
		panic(err)
	}

	code := fmt.Sprintf(tpl, fungibleToken.Hex(), flowToken.Hex(), feeContract.Hex())

	return []byte(code)
}
