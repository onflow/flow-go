package evm

import (
	"math/big"

	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

// Environment is a one-time use flex environment and
// should not be used more than once
// TODO rename to Emulator
type Environment struct {
	Config            *Config
	EVM               *vm.EVM
	Database          *storage.Database
	State             *state.StateDB
	LastExecutedBlock *models.FlexBlock
	Used              bool
}

var _ models.Emulator = &Environment{}

// NewEnvironment constructs a new Flex Enviornment
// TODO: last executed block should be maybe injected here, it should not be a
// concern for the EVM environment to load, should be injected as part of config
func NewEnvironment(
	cfg *Config,
	db *storage.Database,
) (*Environment, error) {

	lastExcutedBlock, err := db.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	execState, err := state.New(lastExcutedBlock.StateRoot,
		state.NewDatabase(
			rawdb.NewDatabase(db),
		),
		nil)
	if err != nil {
		return nil, err
	}

	return &Environment{
		Config: cfg,
		EVM: vm.NewEVM(
			*cfg.BlockContext,
			*cfg.TxContext,
			execState,
			cfg.ChainConfig,
			cfg.EVMConfig,
		),
		Database:          db,
		State:             execState,
		LastExecutedBlock: lastExcutedBlock,
		Used:              false,
	}, nil
}

// BalanceOf returns the balance of an address
// TODO: do it as a ReadOnly env view.
func (fe *Environment) BalanceOf(address models.FlexAddress) (*big.Int, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}
	return fe.State.GetBalance(address.ToCommon()), nil
}

// MintTo mints tokens into the target address, if the address dees not
// exist it would create it first.
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) MintTo(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	var err error
	if err = fe.checkExecuteOnce(); err != nil {
		return nil, err
	}

	faddr := address.ToCommon()
	// update the gas consumed // TODO: revisit
	// do it as the very first thing to prevent attacks
	res := &models.Result{
		GasConsumed: TransferGasUsage,
	}

	// check account if not exist
	if !fe.State.Exist(faddr) {
		fe.State.CreateAccount(faddr)
	}

	// add balance
	fe.State.AddBalance(faddr, amount)

	// we don't need to increment any nonce, given the origin doesn't exist
	res.RootHash, err = fe.commit()

	return res, err
}

// WithdrawFrom deduct the balance from the given source account.
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) WithdrawFrom(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	var err error
	if err = fe.checkExecuteOnce(); err != nil {
		return nil, err
	}

	faddr := address.ToCommon()
	// update the gas consumed // TODO: revisit, we might do this at the higher level
	// do it as the very first thing to prevent attacks
	res := &models.Result{
		GasConsumed: TransferGasUsage,
	}

	// check source balance
	// if balance is lower than amount return
	if fe.State.GetBalance(faddr).Cmp(amount) == -1 {
		return nil, models.ErrInsufficientBalance
	}

	// add balance
	fe.State.SubBalance(faddr, amount)

	// we increment the nonce for source account cause
	// withdraw counts as a transaction (similar to the way calls increment the nonce)
	nonce := fe.State.GetNonce(faddr)
	fe.State.SetNonce(faddr, nonce+1)

	res.RootHash, err = fe.commit()
	return res, err
}

// Transfer transfers flow token from an FOA account to another flex account
// this is a similar functionality as calling a call with empty data,
// mostly provided for a easier interaction
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) Transfer(
	from, to models.FlexAddress,
	value *big.Int,
) (*models.Result, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}
	msg := directCallMessage(from, &to, value, nil, DefaultMaxGasLimit)
	return fe.run(msg)
}

// Deploy deploys a contract at the given address
// the value passed to this method would be deposited on the contract account
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) Deploy(
	caller models.FlexAddress,
	code models.Code,
	gasLimit uint64,
	value *big.Int,
) (*models.Result, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}
	msg := directCallMessage(caller, nil, value, code, gasLimit)
	return fe.run(msg)
}

// Call calls a smart contract with the input
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) Call(
	from, to models.FlexAddress,
	data models.Data,
	gasLimit uint64,
	value *big.Int,
) (*models.Result, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}
	msg := directCallMessage(from, &to, value, data, gasLimit)
	return fe.run(msg)
}

// RunTransaction runs a flex transaction
// this method could be called by anyone.
// TODO : check gas limit complience (one set on tx and the one allowed by flow tx)
func (fe *Environment) RunTransaction(tx *types.Transaction) (*models.Result, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}
	signer := types.MakeSigner(fe.Config.ChainConfig, BlockNumberForEVMRules, fe.Config.BlockContext.Time)

	msg, err := core.TransactionToMessage(tx, signer, fe.Config.BlockContext.BaseFee)
	if err != nil {
		// note that this is not a fatal errro
		return nil, models.NewEVMExecutionError(err)
	}

	return fe.run(msg)
}

func (fe *Environment) run(msg *core.Message) (*models.Result, error) {
	execResult, err := core.NewStateTransition(fe.EVM, msg, (*core.GasPool)(&fe.Config.BlockContext.GasLimit)).TransitionDb()
	if err != nil {
		// this is not a fatal error
		// TODO: we might revist this later
		return nil, models.NewEVMExecutionError(err)
	}

	res := &models.Result{
		ReturnedValue: execResult.ReturnData,
		GasConsumed:   execResult.UsedGas,
		Logs:          fe.State.Logs(),
	}

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To == nil {
		res.DeployedContractAddress = models.NewFlexAddress(crypto.CreateAddress(msg.From, msg.Nonce))
	}

	if execResult.Failed() {
		return nil, models.NewEVMExecutionError(execResult.Err)
	}

	// TODO check if we have the logic to pay the coinbase
	res.RootHash, err = fe.commit()

	return res, err
}

func directCallMessage(
	from models.FlexAddress,
	to *models.FlexAddress,
	value *big.Int,
	data []byte,
	gasLimit uint64,
) *core.Message {
	var t *common.Address
	if to != nil {
		tc := to.ToCommon()
		t = &tc
	}
	return &core.Message{
		From:      from.ToCommon(),
		To:        t,
		Value:     value,
		Data:      data,
		GasLimit:  gasLimit,
		GasPrice:  big.NewInt(0), // price has to be zero
		GasTipCap: big.NewInt(1), // TODO revisit this value (also known as maxPriorityFeePerGas)
		GasFeeCap: big.NewInt(2), // TODO revisit this value (also known as maxFeePerGas)
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}

func (fe *Environment) checkExecuteOnce() error {
	if fe.Used {
		return models.ErrFlexEnvReuse
	}
	fe.Used = true
	return nil
}

// commit commits the changes to the state.
// if error is returned is a fatal one.
func (fe *Environment) commit() (common.Hash, error) {
	// commit the changes
	// ramtin: height is needed when we want to update to version v13
	// var height uint64
	// if fe.Config.BlockContext.BlockNumber != nil {
	// 	height = fe.Config.BlockContext.BlockNumber.Uint64()
	// }

	// commits the changes from the journal into the in memory trie.
	newRoot, err := fe.State.Commit(true)
	if err != nil {
		return types.EmptyRootHash, models.NewFatalError(err)
	}

	// flush the trie to the lower level db
	// the reason we have to do this, is the original database
	// is designed to keep changes in memory until the state.Commit
	// is called then the changes moves into the trie, but the trie
	// would stay in memory for faster trasnaction execution. you
	// have to explicitly ask the trie to commit to the underlying storage
	err = fe.State.Database().TrieDB().Commit(newRoot, false)
	if err != nil {
		return types.EmptyRootHash, models.NewFatalError(err)
	}

	// commit atree changes back to the backend
	err = fe.Database.Commit()
	if err != nil {
		return types.EmptyRootHash, models.NewFatalError(err)
	}

	return newRoot, nil
}

// TODO hid config from upper level and construct EVM per operation
// Question: is it safe to reuse injected db, probably yes, as we construct the
// the execution state again from it.
//
// func getNewEVM(cfg *Config) *vm.EVM {
// 	return vm.NewEVM(
// 		*cfg.BlockContext,
// 		*cfg.TxContext,
// 		execState,
// 		cfg.ChainConfig,
// 		cfg.EVMConfig,
// 	)
// }
