package bootstrap

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-core-contracts/contracts"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Bootstrapper struct {
	logger zerolog.Logger
}

func NewBootstrapper(logger zerolog.Logger) *Bootstrapper {
	return &Bootstrapper{
		logger: logger,
	}
}

// BootstrapLedger adds the above root account to the ledger and initializes execution node-only data
func (b *Bootstrapper) BootstrapLedger(
	ledger storage.Ledger,
	rootPublicKey flow.AccountPublicKey,
	initialTokenSupply uint64,
) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	b.BootstrapView(view, rootPublicKey, initialTokenSupply, false)

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta(), ledger.EmptyStateCommitment())
	if err != nil {
		return nil, err
	}

	return newStateCommitment, nil
}

func (b *Bootstrapper) BootstrapExecutionDatabase(db *badger.DB, commit flow.StateCommitment, genesis *flow.Header) error {
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

func (b *Bootstrapper) BootstrapView(
	ledger virtualmachine.Ledger,
	serviceAccountPublicKey flow.AccountPublicKey,
	initialTokenSupply uint64,
	simpleAddresses bool,
) {
	l := virtualmachine.NewLedgerDAL(ledger, simpleAddresses)

	var addressState AddressState
	if simpleAddresses {
		addressState = &virtualmachine.SimpleAddressState{}
	} else {
		addressState = flow.NewAddressGenerator()
	}

	// initialize the account addressing state
	l.SetAddressState(addressState)

	service := createServiceAccount(ledger, serviceAccountPublicKey, simpleAddresses)

	publicKeyJSON, err := serviceAccountPublicKey.MarshalJSON()
	if err != nil {
		panic(fmt.Sprintf("cannot marshal public key: %s", err))
	}

	b.logger.Debug().Str("account_address", service.Short()).RawJSON("public_key", publicKeyJSON).Msg("created service account")

	rt := runtime.NewInterpreterRuntime()
	vm, err := virtualmachine.New(rt, virtualmachine.WithSimpleAddresses(simpleAddresses))
	if err != nil {
		panic(err)
	}

	ctx := vm.NewBlockContext(nil, nil)

	fungibleToken := deployFungibleToken(ctx, ledger, simpleAddresses)
	flowToken := deployFlowToken(ctx, ledger, simpleAddresses, service, fungibleToken)
	feeContract := deployFlowFees(ctx, ledger, simpleAddresses, service, fungibleToken, flowToken)

	if initialTokenSupply > 0 {
		mintInitialTokens(ctx, ledger, simpleAddresses, service, initialTokenSupply)
	}

	initServiceAccount(ctx, ledger, service, fungibleToken, flowToken, feeContract)
}

func createAccount(ledger virtualmachine.Ledger, simpleAddresses bool) flow.Address {
	l := virtualmachine.NewLedgerDAL(ledger, simpleAddresses)

	addr, err := l.CreateAccount(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return addr
}

func createServiceAccount(ledger virtualmachine.Ledger, accountKey flow.AccountPublicKey, simpleAddresses bool,
) flow.Address {
	l := virtualmachine.NewLedgerDAL(ledger, simpleAddresses)

	addr, err := l.CreateAccount([]flow.AccountPublicKey{accountKey})
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return addr
}

func deployFungibleToken(ctx virtualmachine.BlockContext, ledger virtualmachine.Ledger, simpleAddresses bool,
) flow.Address {
	return deployContract(ctx, ledger, simpleAddresses, contracts.FungibleToken())
}

func deployFlowToken(
	ctx virtualmachine.BlockContext,
	ledger virtualmachine.Ledger,
	simpleAddresses bool,
	service, fungibleToken flow.Address,
) flow.Address {
	flowToken := createAccount(ledger, simpleAddresses)

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
	simpleAddresses bool,
	service, fungibleToken, flowToken flow.Address,
) flow.Address {
	flowFees := createAccount(ledger, simpleAddresses)

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
	simpleAddresses bool,
	service flow.Address,
	initialSupply uint64,
) {
	initialSupplyArg, err := jsoncdc.Encode(cadence.NewUFix64(initialSupply))
	if err != nil {
		panic(fmt.Sprintf("failed to encode initial token supply: %s", err.Error()))
	}

	fungibleTokenAddress := virtualmachine.FungibleTokenAddress()
	flowTokenAddress := virtualmachine.FlowTokenAddress()
	if simpleAddresses {
		fungibleTokenAddress = virtualmachine.SimpleFungibleTokenAddress()
		flowTokenAddress = virtualmachine.SimpleFlowTokenAddress()
	}

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

func deployContract(ctx virtualmachine.BlockContext, ledger virtualmachine.Ledger, simpleAddresses bool,
	contract []byte) flow.Address {
	addr := createAccount(ledger, simpleAddresses)

	script := []byte(
		fmt.Sprintf(`
            transaction {
              prepare(signer: AuthAccount, service: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }
		`, hex.EncodeToString(contract)),
	)

	serviceAddress := flow.ServiceAddress()
	if simpleAddresses {
		serviceAddress = virtualmachine.SimpleServiceAddress()
	}

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
