package emulator

import (
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

// NewBlockView constructs a new block view (mutable)
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
// TODO: allow  multiple calls per block view
// TODO: add block level commit (separation of trie commit to storage)
type BlockView struct {
	config   *Config
	database types.Database
}

// DirectCall executes a direct call
func (bl *BlockView) DirectCall(call *types.DirectCall) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	var res *types.Result
	switch call.SubType {
	case types.DepositCallSubType:
		res, err = proc.mintTo(call.To, call.Value)
	case types.WithdrawCallSubType:
		res, err = proc.withdrawFrom(call.From, call.Value)
	default:
		res, err = proc.run(call.Message(), types.DirectCallTxType)
	}
	if err != nil {
		return res, err
	}
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
		return nil, types.NewEVMValidationError(err)
	}

	// update tx context origin
	proc.evm.TxContext.Origin = msg.From
	res, err := proc.run(msg, tx.Type())
	if err != nil {
		return res, err
	}

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
		state:    execState,
		database: bl.database,
	}, nil
}

func (bl *BlockView) commit(rootHash gethCommon.Hash) error {
	// commit atree changes back to the backend
	err := bl.database.Commit(rootHash)
	return handleCommitError(err)
}

type procedure struct {
	config   *Config
	evm      *gethVM.EVM
	state    *gethState.StateDB
	database types.Database
}

// commit commits the changes to the state.
func (proc *procedure) commit() (gethCommon.Hash, error) {
	// commits the changes from the journal into the in memory trie.
	// in the future if we want to move this to the block level we could use finalize
	// to get the root hash
	newRoot, err := proc.state.Commit(0, true)
	if err != nil {
		return gethTypes.EmptyRootHash, handleCommitError(err)
	}

	// flush the trie to the lower level db
	// the reason we have to do this, is the original database
	// is designed to keep changes in memory until the state.Commit
	// is called then the changes moves into the trie, but the trie
	// would stay in memory for faster transaction execution. you
	// have to explicitly ask the trie to commit to the underlying storage
	err = proc.state.Database().TrieDB().Commit(newRoot, false)
	if err != nil {
		return gethTypes.EmptyRootHash, handleCommitError(err)
	}

	// // remove the read registers (no history tracking)
	// err = proc.database.DeleteAndCleanReadKey()
	// if err != nil {
	// 	return gethTypes.EmptyRootHash, types.NewFatalError(err)
	// }
	return newRoot, nil
}

func handleCommitError(err error) error {
	if err == nil {
		return nil
	}
	// if known types (database errors) don't do anything and return
	if types.IsAFatalError(err) || types.IsADatabaseError(err) {
		return err
	}

	// else is a new fatal error
	return types.NewFatalError(err)
}

func (proc *procedure) mintTo(address types.Address, amount *big.Int) (*types.Result, error) {
	var err error
	addr := address.ToCommon()
	res := &types.Result{
		GasConsumed: proc.config.DirectCallBaseGasUsage,
		TxType:      types.DirectCallTxType,
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
		TxType:      types.DirectCallTxType,
	}

	// check if account exists
	// while this method is only called from bridged accounts
	// it might be the case that someone creates a bridged account
	// and never transfer tokens to and call for withdraw
	// TODO: we might revisit this apporach and
	// 		return res, types.ErrAccountDoesNotExist
	// instead
	if !proc.state.Exist(addr) {
		proc.state.CreateAccount(addr)
	}

	// check the source account balance
	// if balance is lower than amount needed for withdrawal, error out
	if proc.state.GetBalance(addr).Cmp(amount) < 0 {
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

func (proc *procedure) run(msg *gethCore.Message, txType uint8) (*types.Result, error) {
	res := types.Result{
		TxType: txType,
	}

	gasPool := (*gethCore.GasPool)(&proc.config.BlockContext.GasLimit)
	execResult, err := gethCore.NewStateTransition(
		proc.evm,
		msg,
		gasPool,
	).TransitionDb()
	if err != nil {
		res.Failed = true
		// if the error is a fatal error or a non-fatal database error return it
		if types.IsAFatalError(err) || types.IsADatabaseError(err) {
			return &res, err
		}
		// otherwise is a validation error (pre-check failure)
		// no state change, wrap the error and return
		return &res, types.NewEVMValidationError(err)
	}

	// if prechecks are passed, the exec result won't be nil
	if execResult != nil {
		res.GasConsumed = execResult.UsedGas
		if !execResult.Failed() { // collect vm errors
			res.ReturnedValue = execResult.ReturnData
			// If the transaction created a contract, store the creation address in the receipt.
			if msg.To == nil {
				res.DeployedContractAddress = types.NewAddress(gethCrypto.CreateAddress(msg.From, msg.Nonce))
			}
			res.Logs = proc.state.Logs()
		} else {
			res.Failed = true
			err = types.NewEVMExecutionError(execResult.Err)
		}
	}
	var commitErr error
	res.StateRootHash, commitErr = proc.commit()
	if commitErr != nil {
		return &res, commitErr
	}
	return &res, err
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
