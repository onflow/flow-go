package bootstrap

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-core-contracts/contracts"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// BootstrapLedger adds the above root account to the ledger and initializes execution node-only data
func BootstrapLedger(
	ledger storage.Ledger,
	rootPublicKey flow.AccountPublicKey,
	initialTokenSupply uint64,
	chain flow.Chain,
) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	BootstrapView(view, rootPublicKey, initialTokenSupply, chain)

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

type AddressState interface {
	Bytes() []byte
	NextAddress() (flow.Address, error)
	CurrentAddress() flow.Address
}

func BootstrapView(
	ledger virtualmachine.Ledger,
	serviceAccountPublicKey flow.AccountPublicKey,
	initialTokenSupply uint64,
	chain flow.Chain,
) {
	l := virtualmachine.NewLedgerDAL(ledger, chain)

	addressGenerator := chain.NewAddressGenerator()

	// initialize the account addressing state
	l.SetAddressState(addressGenerator)

	service := createServiceAccount(ledger, serviceAccountPublicKey, chain)

	rt := runtime.NewInterpreterRuntime()
	vm, err := virtualmachine.New(rt, chain)
	if err != nil {
		panic(err)
	}

	ctx := vm.NewBlockContext(nil, nil)

	fungibleToken := deployFungibleToken(ctx, ledger, chain)
	flowToken := deployFlowToken(ctx, ledger, service, fungibleToken, chain)
	feeContract := deployFlowFees(ctx, ledger, service, fungibleToken, flowToken, chain)

	if initialTokenSupply > 0 {
		mintInitialTokens(ctx, ledger, service, initialTokenSupply, chain)
	}

	initServiceAccount(ctx, ledger, service, fungibleToken, flowToken, feeContract)
}

func createAccount(ledger virtualmachine.Ledger, chain flow.Chain) flow.Address {
	l := virtualmachine.NewLedgerDAL(ledger, chain)

	addr, err := l.CreateAccount(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return addr
}

func createServiceAccount(ledger virtualmachine.Ledger, accountKey flow.AccountPublicKey, chain flow.Chain,
) flow.Address {
	l := virtualmachine.NewLedgerDAL(ledger, chain)

	addr, err := l.CreateAccount([]flow.AccountPublicKey{accountKey})
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return addr
}

func deployFungibleToken(ctx virtualmachine.BlockContext, ledger virtualmachine.Ledger, chain flow.Chain,
) flow.Address {
	return deployContract(ctx, ledger, chain, contracts.FungibleToken())
}

func deployFlowToken(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	service, fungibleToken flow.Address,
	chain flow.Chain,
) flow.Address {
	flowToken := createAccount(ledger, chain)

	contract := contracts.FlowToken(fungibleToken.Hex())

	tx := flow.NewTransactionBody().
		SetScript(virtualmachine.DeployDefaultTokenTransaction(contract)).
		AddAuthorizer(flowToken).
		AddAuthorizer(service)

	result := executeTransaction(ctx, ledger, tx)
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy default token contract: %s", result.Error.ErrorMessage()))
	}

	return flowToken
}

func deployFlowFees(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	service, fungibleToken, flowToken flow.Address,
	chain flow.Chain,
) flow.Address {
	flowFees := createAccount(ledger, chain)

	contract := contracts.FlowFees(fungibleToken.Hex(), flowToken.Hex())

	tx := flow.NewTransactionBody().
		SetScript(virtualmachine.DeployFlowFeesTransaction(contract)).
		AddAuthorizer(flowFees).
		AddAuthorizer(service)

	result := executeTransaction(ctx, ledger, tx)
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy fees contract: %s", result.Error.ErrorMessage()))
	}

	return flowFees
}

func mintInitialTokens(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	service flow.Address,
	initialSupply uint64,
	chain flow.Chain,
) {
	initialSupplyArg, err := jsoncdc.Encode(cadence.NewUFix64(initialSupply))
	if err != nil {
		panic(fmt.Sprintf("failed to encode initial token supply: %s", err.Error()))
	}

	fungibleTokenAddress := virtualmachine.FungibleTokenAddress(chain)
	flowTokenAddress := virtualmachine.FlowTokenAddress(chain)

	tx := flow.NewTransactionBody().
		SetScript(virtualmachine.MintDefaultTokenTransaction(fungibleTokenAddress, flowTokenAddress)).
		AddArgument(initialSupplyArg).
		AddAuthorizer(service)

	result := executeTransaction(ctx, ledger, tx)
	if result.Error != nil {
		panic(fmt.Sprintf("failed to mint initial token supply: %s", result.Error.ErrorMessage()))
	}
}

func initServiceAccount(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	service, fungibleToken, flowToken, feeContract flow.Address,
) {
	serviceAccountContract := contracts.FlowServiceAccount(fungibleToken.Hex(), flowToken.Hex(), feeContract.Hex())
	deployContractToServiceAccount(ctx, ledger, service, serviceAccountContract)
}

func deployContract(ctx virtualmachine.BlockContext, ledger virtualmachine.Ledger, chain flow.Chain,
	contract []byte) flow.Address {
	addr := createAccount(ledger, chain)

	script := []byte(
		fmt.Sprintf(`
            transaction {
              prepare(signer: AuthAccount, service: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }
		`, hex.EncodeToString(contract)),
	)

	serviceAddress := chain.ServiceAddress()

	tx := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(addr).
		AddAuthorizer(serviceAddress)

	result := executeTransaction(ctx, ledger, tx)
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy contract: %s", result.Error.ErrorMessage()))
	}

	return addr
}

func deployContractToServiceAccount(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	serviceAddress flow.Address,
	contract []byte,
) {
	script := []byte(
		fmt.Sprintf(`
            transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }
		`, hex.EncodeToString(contract)),
	)

	tx := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(serviceAddress)

	result := executeTransaction(ctx, ledger, tx)
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy contract: %s", result.Error.ErrorMessage()))
	}
}

func executeTransaction(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	tx *flow.TransactionBody,
) *virtualmachine.TransactionResult {
	result, err := ctx.ExecuteTransaction(ledger, tx, virtualmachine.WithSignatureVerification(false))
	if err != nil {
		panic(err)
	}

	if result.Error != nil {
		panic(result.Error.ErrorMessage())
	}

	return result
}
