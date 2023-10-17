package evm

import (
	"math"
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

// Emulator handles operations against evm runtime
type Emulator struct {
	Config   *Config
	Database *storage.Database
}

var _ models.Emulator = &Emulator{}

// NewEmulator constructs a new EVM Emulator
func NewEmulator(
	config *Config,
	db *storage.Database,
) *Emulator {
	return &Emulator{
		Config:   config,
		Database: db,
	}
}

// TransferGasUsage returns amount of gas usage for token transfer operation
func (em *Emulator) TransferGasUsage() uint64 {
	return TransferGasUsage
}

// BalanceOf returns the balance of the given account
func (em *Emulator) BalanceOf(address models.FlexAddress) (*big.Int, error) {
	execState, err := em.newState()
	if err != nil {
		return nil, err
	}
	return execState.GetBalance(address.ToCommon()), nil
}

// CodeOf return the code of the given account
func (em *Emulator) CodeOf(address models.FlexAddress) (models.Code, error) {
	execState, err := em.newState()
	if err != nil {
		return nil, err
	}
	return execState.GetCode(address.ToCommon()), nil
}

// MintTo mints tokens into the target address, if the address dees not
// exist it would create it first.
func (em *Emulator) MintTo(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	proc, err := em.newProcedure(em.Config)
	if err != nil {
		return nil, err
	}
	res, err := proc.mintTo(address, amount)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, em.commit(res.StateRootHash)
}

// WithdrawFrom deduct the balance from the given address.
func (em *Emulator) WithdrawFrom(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	proc, err := em.newProcedure(em.Config)
	if err != nil {
		return nil, err
	}
	res, err := proc.withdrawFrom(address, amount)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, em.commit(res.StateRootHash)
}

// Transfer transfers flow token from an FOA account to another flex account
// this is a similar functionality as calling a call with empty data,
// mostly provided for a easier interaction
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (em *Emulator) Transfer(
	from, to models.FlexAddress,
	value *big.Int,
) (*models.Result, error) {
	proc, err := em.newProcedure(NewConfig(WithBlockNumber(BlockNumberForEVMRules)))
	if err != nil {
		return nil, err
	}

	msg := directCallMessage(from, &to, value, nil, math.MaxUint64)

	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, em.commit(res.StateRootHash)
}

// Deploy deploys a contract at the given address
// the value passed to this method would be deposited on the contract account
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (em *Emulator) Deploy(
	caller models.FlexAddress,
	code models.Code,
	gasLimit uint64,
	value *big.Int,
) (*models.Result, error) {
	proc, err := em.newProcedure(em.Config)
	if err != nil {
		return nil, err
	}

	msg := directCallMessage(caller, nil, value, code, gasLimit)

	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, em.commit(res.StateRootHash)
}

// Call calls a smart contract with the input
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (em *Emulator) Call(
	from, to models.FlexAddress,
	data models.Data,
	gasLimit uint64,
	value *big.Int,
) (*models.Result, error) {
	proc, err := em.newProcedure(em.Config)
	if err != nil {
		return nil, err
	}
	msg := directCallMessage(from, &to, value, data, gasLimit)
	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, em.commit(res.StateRootHash)
}

// RunTransaction runs an evm transaction
func (em *Emulator) RunTransaction(tx *types.Transaction, coinbase models.FlexAddress) (*models.Result, error) {

	orgCoinbase := em.Config.BlockContext.Coinbase
	em.Config.BlockContext.Coinbase = coinbase.ToCommon()
	defer func() {
		em.Config.BlockContext.Coinbase = orgCoinbase
	}()

	proc, err := em.newProcedure(em.Config)
	if err != nil {
		return nil, err
	}

	msg, err := core.TransactionToMessage(tx, GetSigner(em.Config), proc.config.BlockContext.BaseFee)
	if err != nil {
		// note that this is not a fatal errro
		return nil, models.NewEVMExecutionError(err)
	}

	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}

	res.TxType = tx.Type()
	return res, em.commit(res.StateRootHash)
}

func (em *Emulator) newState() (*state.StateDB, error) {
	root, err := em.Database.GetRootHash()
	if err != nil {
		return nil, err
	}

	return state.New(root,
		state.NewDatabase(
			rawdb.NewDatabase(em.Database),
		),
		nil)
}

func (em *Emulator) newProcedure(cfg *Config) (*procedure, error) {
	execState, err := em.newState()
	if err != nil {
		return nil, err
	}
	return &procedure{
		config: cfg,
		evm: vm.NewEVM(
			*cfg.BlockContext,
			*cfg.TxContext,
			execState,
			cfg.ChainConfig,
			cfg.EVMConfig,
		),
		state: execState,
	}, nil
}

func (fe *Emulator) commit(rootHash common.Hash) error {
	// sets root hash
	fe.Database.SetRootHash(rootHash)

	// commit atree changes back to the backend
	err := fe.Database.Commit()
	if err != nil {
		return models.NewFatalError(err)
	}

	return nil
}

type procedure struct {
	config *Config
	evm    *vm.EVM
	state  *state.StateDB
}

// commit commits the changes to the state.
// if error is returned is a fatal one.
func (proc *procedure) commit() (common.Hash, error) {
	// commit the changes
	// ramtin: height is needed when we want to update to version v13
	// var height uint64
	// if fe.Config.BlockContext.BlockNumber != nil {
	// 	height = fe.Config.BlockContext.BlockNumber.Uint64()
	// }

	// commits the changes from the journal into the in memory trie.
	newRoot, err := proc.state.Commit(true)
	if err != nil {
		return types.EmptyRootHash, models.NewFatalError(err)
	}

	// flush the trie to the lower level db
	// the reason we have to do this, is the original database
	// is designed to keep changes in memory until the state.Commit
	// is called then the changes moves into the trie, but the trie
	// would stay in memory for faster trasnaction execution. you
	// have to explicitly ask the trie to commit to the underlying storage
	err = proc.state.Database().TrieDB().Commit(newRoot, false)
	if err != nil {
		return types.EmptyRootHash, models.NewFatalError(err)
	}
	return newRoot, nil
}

func (proc *procedure) mintTo(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	var err error
	faddr := address.ToCommon()
	// minting from gas prespective is considered similar to transfer of tokens
	res := &models.Result{
		GasConsumed: TransferGasUsage,
	}

	// check account if not exist
	if !proc.state.Exist(faddr) {
		proc.state.CreateAccount(faddr)
	}

	// add balance
	proc.state.AddBalance(faddr, amount)

	// we don't need to increment any nonce, given the origin doesn't exist
	res.StateRootHash, err = proc.commit()

	return res, err
}

func (proc *procedure) withdrawFrom(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
	var err error

	faddr := address.ToCommon()
	// token withdraw from gas prespective is considered similar to transfer of tokens
	res := &models.Result{
		GasConsumed: TransferGasUsage,
	}

	// check source balance
	// if balance is lower than amount return
	if proc.state.GetBalance(faddr).Cmp(amount) == -1 {
		return nil, models.ErrInsufficientBalance
	}

	// add balance
	proc.state.SubBalance(faddr, amount)

	// we increment the nonce for source account cause
	// withdraw counts as a transaction (similar to the way calls increment the nonce)
	nonce := proc.state.GetNonce(faddr)
	proc.state.SetNonce(faddr, nonce+1)

	res.StateRootHash, err = proc.commit()
	return res, err
}

func (proc *procedure) run(msg *core.Message) (*models.Result, error) {
	execResult, err := core.NewStateTransition(proc.evm, msg, (*core.GasPool)(&proc.config.BlockContext.GasLimit)).TransitionDb()
	if err != nil {
		// this is not a fatal error
		// TODO: we might revist this later
		return nil, models.NewEVMExecutionError(err)
	}

	res := &models.Result{
		ReturnedValue: execResult.ReturnData,
		GasConsumed:   execResult.UsedGas,
		Logs:          proc.state.Logs(),
	}

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To == nil {
		res.DeployedContractAddress = models.NewFlexAddress(crypto.CreateAddress(msg.From, msg.Nonce))
	}

	if execResult.Failed() {
		return nil, models.NewEVMExecutionError(execResult.Err)
	}

	res.StateRootHash, err = proc.commit()

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
		GasPrice:  big.NewInt(0), // price is set to zero fo direct calls
		GasTipCap: big.NewInt(1), // TODO revisit this value (also known as maxPriorityFeePerGas)
		GasFeeCap: big.NewInt(2), // TODO revisit this value (also known as maxFeePerGas)
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}
