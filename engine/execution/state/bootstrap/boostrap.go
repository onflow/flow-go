package bootstrap

import (
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// BootstrapLedger adds the above root account to the ledger and initializes execution node-only data
func BootstrapLedger(ledger storage.Ledger, rootPrivateKey flow.AccountPrivateKey) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	BootstrapView(view, rootPrivateKey)

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

func BootstrapView(ledger virtualmachine.Ledger, rootPrivateKey flow.AccountPrivateKey) {
	root := createRootAccount(ledger, rootPrivateKey)

	rt := runtime.NewInterpreterRuntime()
	vm, err := virtualmachine.New(rt)
	if err != nil {
		panic(err)
	}

	ctx := vm.NewBlockContext(nil)

	fungibleToken := deployFungibleToken(ctx, ledger)
	flowToken := deployFlowToken(ctx, ledger, fungibleToken)
	feeContract := deployFeeContract(ctx, ledger, fungibleToken, flowToken)

	initRootAccount(ctx, ledger, root, fungibleToken, flowToken, feeContract)
}

func createAccount(ledger virtualmachine.Ledger) flow.Address {
	l := virtualmachine.NewLedgerDAL(ledger)

	addr, err := l.CreateAccount(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return addr
}

func createRootAccount(ledger virtualmachine.Ledger, privateKey flow.AccountPrivateKey) flow.Address {
	l := virtualmachine.NewLedgerDAL(ledger)

	accountKey := privateKey.PublicKey(virtualmachine.AccountKeyWeightThreshold)

	err := l.CreateAccountWithAddress(
		flow.RootAddress,
		[]flow.AccountPublicKey{accountKey},
	)

	if err != nil {
		panic(fmt.Sprintf("failed to create root account: %s", err))
	}

	return flow.RootAddress
}

func deployFungibleToken(ctx virtualmachine.BlockContext, ledger virtualmachine.Ledger) flow.Address {
	return deployContract(ctx, ledger, fungibleTokenContract())
}

func deployFlowToken(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	fungibleToken flow.Address,
) flow.Address {
	return deployContract(ctx, ledger, flowTokenContract(fungibleToken))
}

func deployFeeContract(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	fungibleToken, flowToken flow.Address,
) flow.Address {
	return deployContract(ctx, ledger, feeContract(fungibleToken, flowToken))
}

func initRootAccount(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	root, fungibleToken, flowToken, feeContract flow.Address,
) {

	deployContractToAccount(
		ctx, ledger, root, serviceAccountContract(fungibleToken, flowToken, feeContract),
	)

	tx := flow.NewTransactionBody().
		SetScript(virtualmachine.InitDefaultTokenScript).
		AddAuthorizer(root)

	executeTransaction(ctx, ledger, tx)
}

func fungibleTokenContract() string {
	code, err := Asset("contracts/FungibleToken.cdc")
	if err != nil {
		panic(err)
	}

	return hex.EncodeToString(code)
}

func flowTokenContract(fungibleToken flow.Address) string {
	tpl, err := AssetString("contracts/FlowToken.cdc")
	if err != nil {
		panic(err)
	}

	code := fmt.Sprintf(tpl, fungibleToken.Hex())

	return hex.EncodeToString([]byte(code))
}

func feeContract(fungibleToken, flowToken flow.Address) string {
	tpl, err := AssetString("contracts/FeeContract.cdc")
	if err != nil {
		panic(err)
	}

	code := fmt.Sprintf(tpl, fungibleToken.Hex(), flowToken.Hex())

	return hex.EncodeToString([]byte(code))
}

func serviceAccountContract(fungibleToken, flowToken, feeContract flow.Address) string {
	tpl, err := AssetString("contracts/ServiceAccount.cdc")
	if err != nil {
		panic(err)
	}

	code := fmt.Sprintf(tpl, fungibleToken.Hex(), flowToken.Hex(), feeContract.Hex())

	return hex.EncodeToString([]byte(code))
}

func deployContract(ctx virtualmachine.BlockContext, ledger virtualmachine.Ledger, contract string) flow.Address {
	addr := createAccount(ledger)

	script := []byte(
		fmt.Sprintf(`
			transaction {
				prepare(signer: AuthAccount) {
				  signer.setCode("%s".decodeHex())
				}
            }
		`, contract),
	)

	tx := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(addr)

	result := executeTransaction(ctx, ledger, tx)
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy contract: %s", result.Error.ErrorMessage()))
	}

	return addr
}

func deployContractToAccount(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	acct flow.Address,
	contract string,
) {
	script := []byte(
		fmt.Sprintf(`
			transaction {
              	prepare(acct: AuthAccount) {
					 acct.setCode("%s".decodeHex())
              	}
            }
		`, contract),
	)

	tx := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(acct)

	executeTransaction(ctx, ledger, tx)
}

func executeTransaction(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	tx *flow.TransactionBody,
) *virtualmachine.TransactionResult {
	result, err := ctx.ExecuteTransaction(ledger, tx, virtualmachine.SkipVerification)
	if err != nil {
		panic(err)
	}

	if result.Error != nil {
		panic(result.Error.ErrorMessage())
	}

	return result
}
