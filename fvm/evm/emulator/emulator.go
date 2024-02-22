package emulator

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// Emulator handles operations against evm runtime
type Emulator struct {
	rootAddr flow.Address
	ledger   atree.Ledger
}

var _ types.Emulator = &Emulator{}

// NewEmulator constructs a new EVM Emulator
func NewEmulator(
	ledger atree.Ledger,
	rootAddr flow.Address,
) *Emulator {
	return &Emulator{
		rootAddr: rootAddr,
		ledger:   ledger,
	}
}

func newConfig(ctx types.BlockContext) *Config {
	return NewConfig(
		WithChainID(ctx.ChainID),
		WithBlockNumber(new(big.Int).SetUint64(ctx.BlockNumber)),
		WithCoinbase(ctx.GasFeeCollector.ToCommon()),
		WithDirectCallBaseGasUsage(ctx.DirectCallBaseGasUsage),
		WithExtraPrecompiles(ctx.ExtraPrecompiles),
		WithGetBlockHashFunction(ctx.GetHashFunc),
		WithRandom(&ctx.Random),
	)
}

// NewReadOnlyBlockView constructs a new readonly block view
func (em *Emulator) NewReadOnlyBlockView(ctx types.BlockContext) (types.ReadOnlyBlockView, error) {
	execState, err := state.NewStateDB(em.ledger, em.rootAddr)
	return &ReadOnlyBlockView{
		state: execState,
	}, err
}

// NewBlockView constructs a new block view (mutable)
func (em *Emulator) NewBlockView(ctx types.BlockContext) (types.BlockView, error) {
	cfg := newConfig(ctx)
	SetupPrecompile(cfg)
	return &BlockView{
		config:   cfg,
		rootAddr: em.rootAddr,
		ledger:   em.ledger,
	}, nil
}

// ReadOnlyBlockView provides a read only view of a block
// could be used multiple times for queries
type ReadOnlyBlockView struct {
	state types.StateDB
}

// BalanceOf returns the balance of the given address
func (bv *ReadOnlyBlockView) BalanceOf(address types.Address) (*big.Int, error) {
	return bv.state.GetBalance(address.ToCommon()), nil
}

// NonceOf returns the nonce of the given address
func (bv *ReadOnlyBlockView) NonceOf(address types.Address) (uint64, error) {
	return bv.state.GetNonce(address.ToCommon()), nil
}

// CodeOf returns the code of the given address
func (bv *ReadOnlyBlockView) CodeOf(address types.Address) (types.Code, error) {
	return bv.state.GetCode(address.ToCommon()), nil
}

// CodeHashOf returns the code hash of the given address
func (bv *ReadOnlyBlockView) CodeHashOf(address types.Address) ([]byte, error) {
	return bv.state.GetCodeHash(address.ToCommon()).Bytes(), nil
}

// BlockView allows mutation of the evm state as part of a block
//
// TODO: allow  multiple calls per block view
// TODO: add block level commit (separation of trie commit to storage)
type BlockView struct {
	config   *Config
	rootAddr flow.Address
	ledger   atree.Ledger
}

// DirectCall executes a direct call
func (bl *BlockView) DirectCall(call *types.DirectCall) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	txHash, err := call.Hash()
	if err != nil {
		return nil, err
	}
	switch call.SubType {
	case types.DepositCallSubType:
		return proc.mintTo(call.To, call.Value)
	case types.WithdrawCallSubType:
		return proc.withdrawFrom(call.From, call.Value)
	case types.DeployCallSubType:
		if !call.EmptyToField() {
			return proc.deployAt(call.From, call.To, call.Data, call.GasLimit, call.Value)
		}
		fallthrough
	default:
		// TODO: when we support mutiple calls per block, we need
		// to update the value zero here for tx index
		return proc.runDirect(call.Message(), txHash, 0, types.DirectCallTxType)
	}
}

// RunTransaction runs an evm transaction
func (bl *BlockView) RunTransaction(
	tx *gethTypes.Transaction,
) (*types.Result, error) {
	var res *types.Result
	var err error
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	msg, err := gethCore.TransactionToMessage(tx, GetSigner(bl.config), proc.config.BlockContext.BaseFee)
	if err != nil {
		// this is not a fatal error (e.g. due to bad signature)
		// not a valid transaction
		return res, types.NewEVMValidationError(err)
	}

	txHash := tx.Hash()

	// update tx context origin
	proc.evm.TxContext.Origin = msg.From
	// TODO: when we support multiple tx per block we need to update
	// the tx index here to proper value
	res, err = proc.run(msg, txHash, 0, tx.Type())
	if err != nil {
		return nil, err
	}
	res.TxHash = txHash
	return res, err
}

func (bl *BlockView) newProcedure() (*procedure, error) {
	execState, err := state.NewStateDB(bl.ledger, bl.rootAddr)
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

type procedure struct {
	config *Config
	evm    *gethVM.EVM
	state  types.StateDB
}

// commit commits the changes to the state.
func (proc *procedure) commit() error {
	err := proc.state.Commit()
	if err != nil {
		// if known types (state errors) don't do anything and return
		if types.IsAFatalError(err) || types.IsAStateError(err) {
			return err
		}

		// else is a new fatal error
		return types.NewFatalError(err)
	}
	return nil
}

func (proc *procedure) mintTo(address types.Address, amount *big.Int) (*types.Result, error) {
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
	return res, proc.commit()
}

func (proc *procedure) withdrawFrom(address types.Address, amount *big.Int) (*types.Result, error) {

	addr := address.ToCommon()
	res := &types.Result{
		GasConsumed: proc.config.DirectCallBaseGasUsage,
		TxType:      types.DirectCallTxType,
	}

	// check if account exists
	// while this method is only called for COAs
	// it might be the case that someone creates a COA
	// and never transfer tokens to and call for withdraw
	if !proc.state.Exist(addr) {
		proc.state.CreateAccount(addr)
	}

	// check the source account balance
	// if balance is lower than amount needed for withdrawal, error out
	if proc.state.GetBalance(addr).Cmp(amount) < 0 {
		return res, gethCore.ErrInsufficientFundsForTransfer
	}

	// sub balance
	proc.state.SubBalance(addr, amount)

	// we increment the nonce for source account cause
	// withdraw counts as a transaction
	nonce := proc.state.GetNonce(addr)
	proc.state.SetNonce(addr, nonce+1)

	return res, proc.commit()
}

// deployAt deploys a contract at the given target address
// behaviour should be similar to what evm.create internal method does with
// a few differences, don't need to check for previous forks given this
// functionality was not available to anyone, we don't need to
// follow snapshoting, given we do commit/revert style in this code base.
// in the future we might optimize this method accepting deploy-ready byte codes
// and skip interpreter call, gas calculations and many checks.
func (proc *procedure) deployAt(
	caller types.Address,
	to types.Address,
	data types.Code,
	gasLimit uint64,
	value *big.Int,
) (*types.Result, error) {
	if value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	res := &types.Result{
		TxType: types.DirectCallTxType,
	}
	addr := to.ToCommon()

	// precheck 1 - check balance of the source
	if value.Sign() != 0 &&
		!proc.evm.Context.CanTransfer(proc.state, caller.ToCommon(), value) {
		return res, gethCore.ErrInsufficientFundsForTransfer
	}

	// precheck 2 - ensure there's no existing eoa or contract is deployed at the address
	contractHash := proc.state.GetCodeHash(addr)
	if proc.state.GetNonce(addr) != 0 ||
		(contractHash != (gethCommon.Hash{}) && contractHash != gethTypes.EmptyCodeHash) {
		res.VMError = gethVM.ErrContractAddressCollision
		return res, nil
	}

	callerCommon := caller.ToCommon()
	// setup caller if doesn't exist
	if !proc.state.Exist(callerCommon) {
		proc.state.CreateAccount(callerCommon)
	}
	// increment the nonce for the caller
	proc.state.SetNonce(callerCommon, proc.state.GetNonce(callerCommon)+1)

	// setup account
	proc.state.CreateAccount(addr)
	proc.state.SetNonce(addr, 1) // (EIP-158)
	if value.Sign() > 0 {
		proc.evm.Context.Transfer( // transfer value
			proc.state,
			caller.ToCommon(),
			addr,
			value,
		)
	}

	// run code through interpreter
	// this would check for errors and computes the final bytes to be stored under account
	var err error
	inter := gethVM.NewEVMInterpreter(proc.evm)
	contract := gethVM.NewContract(
		gethVM.AccountRef(caller.ToCommon()),
		gethVM.AccountRef(addr),
		value,
		gasLimit)

	contract.SetCallCode(&addr, gethCrypto.Keccak256Hash(data), data)
	// update access list (Berlin)
	proc.state.AddAddressToAccessList(addr)

	ret, err := inter.Run(contract, nil, false)
	gasCost := uint64(len(ret)) * gethParams.CreateDataGas
	res.GasConsumed = gasCost

	// handle errors
	if err != nil {
		// for all errors except this one consume all the remaining gas (Homestead)
		if err != gethVM.ErrExecutionReverted {
			res.GasConsumed = gasLimit
		}
		res.VMError = err
		return res, nil
	}

	// update gas usage
	if gasCost > gasLimit {
		// consume all the remaining gas (Homestead)
		res.GasConsumed = gasLimit
		res.VMError = gethVM.ErrCodeStoreOutOfGas
		return res, nil
	}

	// check max code size (EIP-158)
	if len(ret) > gethParams.MaxCodeSize {
		// consume all the remaining gas (Homestead)
		res.GasConsumed = gasLimit
		res.VMError = gethVM.ErrMaxCodeSizeExceeded
		return res, nil
	}

	// reject code starting with 0xEF (EIP-3541)
	if len(ret) >= 1 && ret[0] == 0xEF {
		// consume all the remaining gas (Homestead)
		res.GasConsumed = gasLimit
		res.VMError = gethVM.ErrInvalidCode
		return res, nil
	}

	proc.state.SetCode(addr, ret)
	res.DeployedContractAddress = to
	return res, proc.commit()
}

func (proc *procedure) runDirect(
	msg *gethCore.Message,
	txHash gethCommon.Hash,
	txIndex uint,
	txType uint8,
) (*types.Result, error) {
	// set the nonce for the message (needed for some opeartions like deployment)
	msg.Nonce = proc.state.GetNonce(msg.From)
	proc.evm.TxContext.Origin = msg.From
	return proc.run(msg, txHash, txIndex, types.DirectCallTxType)
}

func (proc *procedure) run(
	msg *gethCore.Message,
	txHash gethCommon.Hash,
	txIndex uint,
	txType uint8,
) (*types.Result, error) {
	res := types.Result{
		TxType: txType,
		TxHash: txHash,
	}

	gasPool := (*gethCore.GasPool)(&proc.config.BlockContext.GasLimit)
	execResult, err := gethCore.NewStateTransition(
		proc.evm,
		msg,
		gasPool,
	).TransitionDb()
	if err != nil {
		// if the error is a fatal error or a non-fatal state error return it
		// this condition should never happen
		// given all StateDB errors are withheld for the commit time.
		if types.IsAFatalError(err) || types.IsAStateError(err) {
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
			// replace tx index and tx hash
			res.Logs = proc.state.Logs(
				proc.config.BlockContext.BlockNumber.Uint64(),
				txHash,
				txIndex,
			)
		} else {
			// execResult.Err is VM errors (we don't return it as error)
			res.VMError = execResult.Err
		}
	}
	// all commmit errors (StateDB errors) has to be returned
	// TODO: maybe handle them (if there are happy errors)
	return &res, proc.commit()
}

func SetupPrecompile(cfg *Config) {
	rules := cfg.ChainRules()
	// captures the pointer to the map that has to be augmented
	var precompiles map[gethCommon.Address]gethVM.PrecompiledContract
	switch {
	case rules.IsCancun:
		precompiles = gethVM.PrecompiledContractsCancun
	case rules.IsBerlin:
		precompiles = gethVM.PrecompiledContractsBerlin
	case rules.IsIstanbul:
		precompiles = gethVM.PrecompiledContractsIstanbul
	case rules.IsByzantium:
		precompiles = gethVM.PrecompiledContractsByzantium
	default:
		precompiles = gethVM.PrecompiledContractsHomestead
	}
	for addr, contract := range cfg.ExtraPrecompiles {
		// we override if exist since we call this method on every block
		precompiles[addr] = contract
	}
}
