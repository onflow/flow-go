package emulator

import (
	"math"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	gethState "github.com/ethereum/go-ethereum/core/state"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// Emulator handles operations against evm runtime
type Emulator struct {
	Database types.Database
}

var _ types.Emulator = &Emulator{}

// NewEmulator constructs a new EVM Emulator
func NewEmulator(
	db types.Database,
) *Emulator {
	return &Emulator{
		Database: db,
	}
}

func newConfig(ctx types.BlockContext) *Config {
	return NewConfig(
		WithBlockNumber(new(big.Int).SetUint64(ctx.BlockNumber)),
		WithCoinbase(ctx.GasFeeCollector.ToCommon()),
		WithDirectCallBaseGasUsage(ctx.DirectCallBaseGasUsage),
	)
}

// NewReadOnlyBlockView constructs a new readonly block view
func (em *Emulator) NewReadOnlyBlockView(ctx types.BlockContext) (types.ReadOnlyBlockView, error) {
	execState, err := newState(em.Database)
	return &ReadOnlyBlockView{
		state: execState,
	}, err
}

// NewBlockView constructs a new block (mutable)
func (em *Emulator) NewBlockView(ctx types.BlockContext) (types.BlockView, error) {
	cfg := newConfig(ctx)
	return &BlockView{
		config:   cfg,
		database: em.Database,
	}, nil
}

// ReadOnlyBlockView provides a read only view of a block
// could be used multiple times for queries
type ReadOnlyBlockView struct {
	state *gethState.StateDB
}

// BalanceOf returns the balance of the given address
func (bv *ReadOnlyBlockView) BalanceOf(address types.Address) (*big.Int, error) {
	return bv.state.GetBalance(address.ToCommon()), nil
}

// CodeOf returns the code of the given address
func (bv *ReadOnlyBlockView) CodeOf(address types.Address) (types.Code, error) {
	return bv.state.GetCode(address.ToCommon()), nil
}

// NonceOf returns the nonce of the given address
func (bv *ReadOnlyBlockView) NonceOf(address types.Address) (uint64, error) {
	return bv.state.GetNonce(address.ToCommon()), nil
}

// BlockView allows mutation of the evm state as part of a block
//
// TODO: allow block level to do multiple procedure per block
// TODO: add block commit
type BlockView struct {
	config   *Config
	database types.Database
}

// MintTo mints tokens into the target address, if the address dees not
// exist it would create it first.
func (bl *BlockView) MintTo(
	address types.Address,
	amount *big.Int,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	res, err := proc.mintTo(address, amount)
	if err != nil {
		return nil, err
	}
	res.TxType = types.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// WithdrawFrom deduct the balance from the given address.
func (bl *BlockView) WithdrawFrom(
	address types.Address,
	amount *big.Int,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	res, err := proc.withdrawFrom(address, amount)
	if err != nil {
		return nil, err
	}
	res.TxType = types.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// Transfer transfers flow token from an bridged account to another account
// this is a similar functionality as calling a call with empty data,
// mostly provided for a easier interaction
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (bl *BlockView) Transfer(
	from, to types.Address,
	value *big.Int,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	msg := directCallMessage(from, &to, value, nil, math.MaxUint64)
	res, err := proc.run(msg)
	if err != nil {
		return res, err
	}
	res.TxType = types.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// Deploy deploys a contract at the given address
// the value passed to this method would be deposited on the contract account
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (bl *BlockView) Deploy(
	caller types.Address,
	code types.Code,
	gasLimit uint64,
	value *big.Int,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	msg := directCallMessage(caller, nil, value, code, gasLimit)
	res, err := proc.run(msg)
	if err != nil {
		return res, err
	}
	res.TxType = types.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// Call calls a smart contract with the input
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (bl *BlockView) Call(
	from, to types.Address,
	data types.Data,
	gasLimit uint64,
	value *big.Int,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	msg := directCallMessage(from, &to, value, data, gasLimit)
	res, err := proc.run(msg)
	if err != nil {
		return res, err
	}
	res.TxType = types.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// RunTransaction runs an evm transaction
func (bl *BlockView) RunTransaction(
	tx *gethTypes.Transaction,
) (*types.Result, error) {
	var err error
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	msg, err := gethCore.TransactionToMessage(tx, GetSigner(bl.config), proc.config.BlockContext.BaseFee)
	if err != nil {
		// note that this is not a fatal error (e.g. due to bad signature)
		// not a valid transaction
		return nil, types.NewEVMExecutionError(err)
	}

	// update tx context origin
	proc.evm.TxContext.Origin = msg.From

	res, err := proc.run(msg)
	if err != nil {
		return res, err
	}

	res.TxType = tx.Type()
	return res, bl.commit(res.StateRootHash)
}

func (bl *BlockView) newProcedure() (*procedure, error) {
	execState, err := newState(bl.database)
	if err != nil {
		return nil, err
	}
	cfg := bl.config
	return &procedure{
		config: cfg,
		evm: gethVM.NewEVM(
			*cfg.BlockContext,
			*cfg.TxContext,
			execState,
			cfg.ChainConfig,
			cfg.EVMConfig,
		),
		state: execState,
	}, nil
}

// TODO errors returns here might not be fatal, what if we have hit the interaction limit ?
func (bl *BlockView) commit(rootHash gethCommon.Hash) error {
	// sets root hash
	err := bl.database.SetRootHash(rootHash)
	if err != nil {
		return types.NewFatalError(err)
	}

	// commit atree changes back to the backend
	err = bl.database.Commit()
	if err != nil {
		return types.NewFatalError(err)
	}

	return nil
}

type procedure struct {
	config *Config
	evm    *gethVM.EVM
	state  *gethState.StateDB
}

// commit commits the changes to the state.
// if error is returned is a fatal one.
func (proc *procedure) commit() (gethCommon.Hash, error) {
	// commits the changes from the journal into the in memory trie.
	// in the future if we want to move this to the block level we could use finalize
	// to get the root hash
	newRoot, err := proc.state.Commit(true)
	if err != nil {
		return gethTypes.EmptyRootHash, types.NewFatalError(err)
	}

	// flush the trie to the lower level db
	// the reason we have to do this, is the original database
	// is designed to keep changes in memory until the state.Commit
	// is called then the changes moves into the trie, but the trie
	// would stay in memory for faster trasnaction execution. you
	// have to explicitly ask the trie to commit to the underlying storage
	err = proc.state.Database().TrieDB().Commit(newRoot, false)
	if err != nil {
		return gethTypes.EmptyRootHash, types.NewFatalError(err)
	}
	return newRoot, nil
}

func (proc *procedure) mintTo(address types.Address, amount *big.Int) (*types.Result, error) {
	var err error
	addr := address.ToCommon()
	res := &types.Result{
		GasConsumed: proc.config.DirectCallBaseGasUsage,
	}

	// create account if not exist
	if !proc.state.Exist(addr) {
		proc.state.CreateAccount(addr)
	}

	// add balance
	proc.state.AddBalance(addr, amount)

	// we don't need to increment any nonce, given the origin doesn't exist
	res.StateRootHash, err = proc.commit()

	return res, err
}

func (proc *procedure) withdrawFrom(address types.Address, amount *big.Int) (*types.Result, error) {
	var err error

	addr := address.ToCommon()
	res := &types.Result{
		GasConsumed: proc.config.DirectCallBaseGasUsage,
	}

	// check source balance
	// if balance is lower than amount return
	if proc.state.GetBalance(addr).Cmp(amount) == -1 {
		return res, types.ErrInsufficientBalance
	}

	// sub balance
	proc.state.SubBalance(addr, amount)

	// we increment the nonce for source account cause
	// withdraw counts as a transaction
	nonce := proc.state.GetNonce(addr)
	proc.state.SetNonce(addr, nonce+1)

	res.StateRootHash, err = proc.commit()
	return res, err
}

func (proc *procedure) run(msg *gethCore.Message) (*types.Result, error) {
	var res types.Result

	gasPool := (*gethCore.GasPool)(&proc.config.BlockContext.GasLimit)
	execResult, err := gethCore.NewStateTransition(
		proc.evm,
		msg,
		gasPool,
	).TransitionDb()

	// if the error is a fatal error don't move forward
	if err != nil {
		if types.IsAFatalError(err) {
			return &res, err
		}
		// when pre-checks has failed (non-fatal errors)
		res.Failed = true
		res.ErrorMessage = err.Error()
		err = types.NewEVMExecutionError(err)
	}

	// if prechecks are passed, the exec result won't be nil
	if execResult != nil {
		res.ReturnedValue = execResult.ReturnData
		res.GasConsumed = execResult.UsedGas
		res.Logs = proc.state.Logs()
		if execResult.Failed() { // collect vm errors
			res.Failed = true
			res.ErrorMessage = execResult.Err.Error()
		}
		// If the transaction created a contract, store the creation address in the receipt.
		if msg.To == nil {
			res.DeployedContractAddress = types.NewAddress(gethCrypto.CreateAddress(msg.From, msg.Nonce))
		}
	}

	res.StateRootHash, err = proc.commit()
	return &res, err
}

func directCallMessage(
	from types.Address,
	to *types.Address,
	value *big.Int,
	data []byte,
	gasLimit uint64,
) *gethCore.Message {
	var t *gethCommon.Address
	if to != nil {
		tc := to.ToCommon()
		t = &tc
	}
	return &gethCore.Message{
		From:      from.ToCommon(),
		To:        t,
		Value:     value,
		Data:      data,
		GasLimit:  gasLimit,
		GasPrice:  big.NewInt(0), // price is set to zero fo direct calls // TODO update me from the config
		GasTipCap: big.NewInt(1), // TODO revisit this value (also known as maxPriorityFeePerGas)
		GasFeeCap: big.NewInt(2), // TODO revisit this value (also known as maxFeePerGas)
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}

// Ramtin: this is the part of the code that we have to update if we hit performance problems
// the NewDatabase from the RawDB might have to change.
func newState(database types.Database) (*gethState.StateDB, error) {
	root, err := database.GetRootHash()
	if err != nil {
		return nil, err
	}

	return gethState.New(root,
		gethState.NewDatabase(
			gethRawDB.NewDatabase(database),
		),
		nil)
}
