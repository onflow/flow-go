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
	Database *storage.Database
}

var _ models.Emulator = &Emulator{}

// NewEmulator constructs a new EVM Emulator
func NewEmulator(
	db *storage.Database,
) *Emulator {
	return &Emulator{
		Database: db,
	}
}

func newConfig(ctx models.BlockContext) *Config {
	return NewConfig(
		WithBlockNumber(new(big.Int).SetUint64(ctx.BlockNumber)),
		WithCoinbase(ctx.GasFeeCollector.ToCommon()),
		WithDirectCallBaseGasUsage(ctx.DirectCallBaseGasUsage),
	)
}

// BlockView constructs a new block view
func (em *Emulator) NewBlockView(ctx models.BlockContext) (models.BlockView, error) {
	execState, err := newState(em.Database)
	return &BlockView{
		state: execState,
	}, err
}

// BlockView constructs a new block
func (em *Emulator) NewBlock(ctx models.BlockContext) (models.Block, error) {
	cfg := newConfig(ctx)
	return &Block{
		config:   cfg,
		database: em.Database,
	}, nil
}

type BlockView struct {
	state *state.StateDB
}

// BalanceOf returns the balance of the given account
func (bv *BlockView) BalanceOf(address models.FlexAddress) (*big.Int, error) {
	return bv.state.GetBalance(address.ToCommon()), nil
}

// CodeOf return the code of the given account
func (bv *BlockView) CodeOf(address models.FlexAddress) (models.Code, error) {
	return bv.state.GetCode(address.ToCommon()), nil
}

// TODO: allow block level to do multiple procedure per block
// TODO: add block commit
type Block struct {
	config   *Config
	database *storage.Database
}

// MintTo mints tokens into the target address, if the address dees not
// exist it would create it first.
func (bl *Block) MintTo(
	address models.FlexAddress,
	amount *big.Int,
) (*models.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	res, err := proc.mintTo(address, amount)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// WithdrawFrom deduct the balance from the given address.
func (bl *Block) WithdrawFrom(
	address models.FlexAddress,
	amount *big.Int,
) (*models.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	res, err := proc.withdrawFrom(address, amount)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// Transfer transfers flow token from an FOA account to another flex account
// this is a similar functionality as calling a call with empty data,
// mostly provided for a easier interaction
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (bl *Block) Transfer(
	from, to models.FlexAddress,
	value *big.Int,
) (*models.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	msg := directCallMessage(from, &to, value, nil, math.MaxUint64)

	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// Deploy deploys a contract at the given address
// the value passed to this method would be deposited on the contract account
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (bl *Block) Deploy(
	caller models.FlexAddress,
	code models.Code,
	gasLimit uint64,
	value *big.Int,
) (*models.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	msg := directCallMessage(caller, nil, value, code, gasLimit)

	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// Call calls a smart contract with the input
//
// Warning, This method should only be used for bridged accounts
// where resource ownership has been verified
func (bl *Block) Call(
	from, to models.FlexAddress,
	data models.Data,
	gasLimit uint64,
	value *big.Int,
) (*models.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	msg := directCallMessage(from, &to, value, data, gasLimit)
	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}
	res.TxType = models.DirectCallTxType
	return res, bl.commit(res.StateRootHash)
}

// RunTransaction runs an evm transaction
func (bl *Block) RunTransaction(
	tx *types.Transaction,
) (*models.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	msg, err := core.TransactionToMessage(tx, GetSigner(bl.config), proc.config.BlockContext.BaseFee)
	if err != nil {
		// note that this is not a fatal errro
		return nil, models.NewEVMExecutionError(err)
	}

	res, err := proc.run(msg)
	if err != nil {
		return nil, err
	}

	res.TxType = tx.Type()
	return res, bl.commit(res.StateRootHash)
}

func (bl *Block) newProcedure() (*procedure, error) {
	execState, err := newState(bl.database)
	if err != nil {
		return nil, err
	}
	cfg := bl.config
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

func (bl *Block) commit(rootHash common.Hash) error {
	// sets root hash
	bl.database.SetRootHash(rootHash)

	// commit atree changes back to the backend
	err := bl.database.Commit()
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
	// commits the changes from the journal into the in memory trie.
	// in the future if we want to move this to the block level we could use finalize
	// to get the root hash
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
		GasConsumed: proc.config.DirectCallBaseGasUsage,
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
		GasConsumed: proc.config.DirectCallBaseGasUsage,
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
		GasPrice:  big.NewInt(0), // price is set to zero fo direct calls // TODO update me from the config
		GasTipCap: big.NewInt(1), // TODO revisit this value (also known as maxPriorityFeePerGas)
		GasFeeCap: big.NewInt(2), // TODO revisit this value (also known as maxFeePerGas)
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}

func newState(database *storage.Database) (*state.StateDB, error) {
	root, err := database.GetRootHash()
	if err != nil {
		return nil, err
	}

	return state.New(root,
		state.NewDatabase(
			rawdb.NewDatabase(database),
		),
		nil)
}
