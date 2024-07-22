package emulator

import (
	"errors"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/onflow/atree"
	"github.com/onflow/go-ethereum/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	"github.com/onflow/go-ethereum/core/tracing"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	gethParams "github.com/onflow/go-ethereum/params"

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
		WithBlockTime(ctx.BlockTimestamp),
		WithCoinbase(ctx.GasFeeCollector.ToCommon()),
		WithDirectCallBaseGasUsage(ctx.DirectCallBaseGasUsage),
		WithExtraPrecompiledContracts(ctx.ExtraPrecompiledContracts),
		WithGetBlockHashFunction(ctx.GetHashFunc),
		WithRandom(&ctx.Random),
		WithTransactionTracer(ctx.Tracer),
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
	bal := bv.state.GetBalance(address.ToCommon())
	return bal.ToBig(), nil
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
type BlockView struct {
	config   *Config
	rootAddr flow.Address
	ledger   atree.Ledger
}

// DirectCall executes a direct call
func (bl *BlockView) DirectCall(call *types.DirectCall) (*types.Result, error) {
	// negative amounts are not acceptable.
	if call.Value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	txHash := call.Hash()
	switch call.SubType {
	case types.DepositCallSubType:
		return proc.mintTo(call, txHash)
	case types.WithdrawCallSubType:
		return proc.withdrawFrom(call, txHash)
	case types.DeployCallSubType:
		if !call.EmptyToField() {
			return proc.deployAt(call.From, call.To, call.Data, call.GasLimit, call.Value, txHash)
		}
		fallthrough
	default:
		return proc.runDirect(call.Message(), txHash, 0)
	}
}

// RunTransaction runs an evm transaction
func (bl *BlockView) RunTransaction(
	tx *gethTypes.Transaction,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	msg, err := gethCore.TransactionToMessage(
		tx,
		GetSigner(bl.config),
		proc.config.BlockContext.BaseFee)
	if err != nil {
		// this is not a fatal error (e.g. due to bad signature)
		// not a valid transaction
		return types.NewInvalidResult(tx, err), nil
	}

	// negative amounts are not acceptable.
	if msg.Value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	// update tx context origin
	proc.evm.TxContext.Origin = msg.From
	res, err := proc.run(msg, tx.Hash(), 0, tx.Type())
	if err != nil {
		return nil, err
	}
	// all commit errors (StateDB errors) has to be returned
	if err := proc.commit(true); err != nil {
		return nil, err
	}

	return res, nil
}

func (bl *BlockView) BatchRunTransactions(txs []*gethTypes.Transaction) ([]*types.Result, error) {
	batchResults := make([]*types.Result, len(txs))

	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	for i, tx := range txs {
		msg, err := gethCore.TransactionToMessage(
			tx,
			GetSigner(bl.config),
			proc.config.BlockContext.BaseFee)
		if err != nil {
			batchResults[i] = types.NewInvalidResult(tx, err)
			continue
		}

		// negative amounts are not acceptable.
		if msg.Value.Sign() < 0 {
			return nil, types.ErrInvalidBalance
		}

		// update tx context origin
		proc.evm.TxContext.Origin = msg.From
		res, err := proc.run(msg, tx.Hash(), uint(i), tx.Type())
		if err != nil {
			return nil, err
		}
		// all commit errors (StateDB errors) has to be returned
		if err := proc.commit(false); err != nil {
			return nil, err
		}

		// this clears state for any subsequent transaction runs
		proc.state.Reset()

		batchResults[i] = res
	}

	// finalize after all the batch transactions are executed to save resources
	if err := proc.state.Finalize(); err != nil {
		return nil, err
	}

	return batchResults, nil
}

// DryRunTransaction run unsigned transaction without persisting the state
func (bl *BlockView) DryRunTransaction(
	tx *gethTypes.Transaction,
	from gethCommon.Address,
) (*types.Result, error) {
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	msg, err := gethCore.TransactionToMessage(
		tx,
		GetSigner(bl.config),
		proc.config.BlockContext.BaseFee,
	)
	// negative amounts are not acceptable.
	if msg.Value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	// we can ignore invalid signature errors since we don't expect signed transctions
	if !errors.Is(err, gethTypes.ErrInvalidSig) {
		return nil, err
	}

	// use the from as the signer
	proc.evm.TxContext.Origin = from
	msg.From = from
	// we need to skip nonce check for dry run
	msg.SkipAccountChecks = true

	// return without commiting the state
	txResult, err := proc.run(msg, tx.Hash(), 0, tx.Type())
	if txResult.Successful() {
		// As mentioned in https://github.com/ethereum/EIPs/blob/master/EIPS/eip-150.md#specification
		// Define "all but one 64th" of N as N - floor(N / 64).
		// If a call asks for more gas than the maximum allowed amount
		// (i.e. the total amount of gas remaining in the parent after subtracting
		// the gas cost of the call and memory expansion), do not return an OOG error;
		// instead, if a call asks for more gas than all but one 64th of the maximum
		// allowed amount, call with all but one 64th of the maximum allowed amount of
		// gas (this is equivalent to a version of EIP-901 plus EIP-1142).
		// CREATE only provides all but one 64th of the parent gas to the child call.
		txResult.GasConsumed = AddOne64th(txResult.GasConsumed)

		// Adding `gethParams.SstoreSentryGasEIP2200` is needed for this condition:
		// https://github.com/onflow/go-ethereum/blob/master/core/vm/operations_acl.go#L29-L32
		txResult.GasConsumed += gethParams.SstoreSentryGasEIP2200

		// Take into account any gas refunds, which are calculated only after
		// transaction execution.
		txResult.GasConsumed += txResult.GasRefund
	}

	return txResult, err
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

// commit commits the changes to the state (with optional finalization)
func (proc *procedure) commit(finalize bool) error {
	err := proc.state.Commit(finalize)
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

func (proc *procedure) mintTo(
	call *types.DirectCall,
	txHash gethCommon.Hash,
) (*types.Result, error) {
	// negative amounts are not acceptable.
	if call.Value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	value, overflow := uint256.FromBig(call.Value)
	if overflow {
		return nil, types.ErrInvalidBalance
	}

	bridge := call.From.ToCommon()

	// create bridge account if not exist
	if !proc.state.Exist(bridge) {
		proc.state.CreateAccount(bridge)
	}

	// add balance to the bridge account before transfer
	proc.state.AddBalance(bridge, value, tracing.BalanceIncreaseWithdrawal)

	msg := call.Message()
	proc.evm.TxContext.Origin = msg.From
	// withdraw the amount and move it to the bridge account
	res, err := proc.run(msg, txHash, 0, types.DirectCallTxType)
	if err != nil {
		return res, err
	}

	// if any error (invalid or vm) on the internal call, revert and don't commit any change
	// this prevents having cases that we add balance to the bridge but the transfer
	// fails due to gas, etc.
	// TODO: in the future we might just return without error and handle everything on higher level
	if res.Invalid() || res.Failed() {
		return res, types.ErrInternalDirectCallFailed
	}

	// all commmit errors (StateDB errors) has to be returned
	return res, proc.commit(true)
}

func (proc *procedure) withdrawFrom(
	call *types.DirectCall,
	txHash gethCommon.Hash,
) (*types.Result, error) {
	// negative amounts are not acceptable.
	if call.Value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	value, overflow := uint256.FromBig(call.Value)
	if overflow {
		return nil, types.ErrInvalidBalance
	}

	bridge := call.To.ToCommon()

	// create bridge account if not exist
	if !proc.state.Exist(bridge) {
		proc.state.CreateAccount(bridge)
	}

	// withdraw the amount and move it to the bridge account
	msg := call.Message()
	proc.evm.TxContext.Origin = msg.From
	res, err := proc.run(msg, txHash, 0, types.DirectCallTxType)
	if err != nil {
		return res, err
	}

	// if any error (invalid or vm) on the internal call, revert and don't commit any change
	// TODO: in the future we might just return without error and handle everything on higher level
	if res.Invalid() || res.Failed() {
		return res, types.ErrInternalDirectCallFailed
	}

	// now deduct the balance from the bridge
	proc.state.SubBalance(bridge, value, tracing.BalanceIncreaseWithdrawal)
	// all commmit errors (StateDB errors) has to be returned
	return res, proc.commit(true)
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
	txHash gethCommon.Hash,
) (*types.Result, error) {
	if value.Sign() < 0 {
		return nil, types.ErrInvalidBalance
	}

	castedValue, overflow := uint256.FromBig(value)
	if overflow {
		return nil, types.ErrInvalidBalance
	}

	res := &types.Result{
		TxType: types.DirectCallTxType,
		TxHash: txHash,
	}

	if proc.evm.Config.Tracer != nil {
		proc.captureTraceBegin(0, gethVM.CREATE2, caller.ToCommon(), to.ToCommon(), data, gasLimit, value)
		defer proc.captureTraceEnd(0, gasLimit, res.ReturnedData, res.Receipt(0), res.VMError)
	}

	addr := to.ToCommon()
	// precheck 1 - check balance of the source
	if value.Sign() != 0 &&
		!proc.evm.Context.CanTransfer(proc.state, caller.ToCommon(), castedValue) {
		res.SetValidationError(gethCore.ErrInsufficientFundsForTransfer)
		return res, nil
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
			uint256.MustFromBig(value),
		)
	}

	// run code through interpreter
	// this would check for errors and computes the final bytes to be stored under account
	var err error
	inter := gethVM.NewEVMInterpreter(proc.evm)
	contract := gethVM.NewContract(
		gethVM.AccountRef(caller.ToCommon()),
		gethVM.AccountRef(addr),
		castedValue,
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

	res.DeployedContractAddress = &to

	proc.state.SetCode(addr, ret)
	return res, proc.commit(true)
}

func (proc *procedure) runDirect(
	msg *gethCore.Message,
	txHash gethCommon.Hash,
	txIndex uint,
) (*types.Result, error) {
	// set the nonce for the message (needed for some operations like deployment)
	msg.Nonce = proc.state.GetNonce(msg.From)
	proc.evm.TxContext.Origin = msg.From
	res, err := proc.run(msg, txHash, txIndex, types.DirectCallTxType)
	if err != nil {
		return nil, err
	}
	// all commit errors (StateDB errors) has to be returned
	return res, proc.commit(true)
}

// run runs a geth core.message and returns the
// results, any validation or execution errors
// are captured inside the result, the remaining
// return errors are errors requires extra handling
// on upstream (e.g. backend errors).
func (proc *procedure) run(
	msg *gethCore.Message,
	txHash gethCommon.Hash,
	txIndex uint,
	txType uint8,
) (*types.Result, error) {
	var err error
	res := types.Result{
		TxType: txType,
		TxHash: txHash,
	}

	// reset precompile tracking in case
	proc.config.PCTracker.Reset()
	gasPool := (*gethCore.GasPool)(&proc.config.BlockContext.GasLimit)
	execResult, err := gethCore.NewStateTransition(
		proc.evm,
		msg,
		gasPool,
	).TransitionDb()
	if err != nil {
		// if the error is a fatal error or a non-fatal state error or a backend err return it
		// this condition should never happen given all StateDB errors are withheld for the commit time.
		if types.IsAFatalError(err) || types.IsAStateError(err) || types.IsABackendError(err) {
			return nil, err
		}
		// otherwise is a validation error (pre-check failure)
		// no state change, wrap the error and return
		res.SetValidationError(err)
		return &res, nil
	}

	// if prechecks are passed, the exec result won't be nil
	if execResult != nil {
		res.GasConsumed = execResult.UsedGas
		res.GasRefund = proc.state.GetRefund()
		res.Index = uint16(txIndex)
		res.PrecompiledCalls, err = proc.config.PCTracker.CapturedCalls()
		if err != nil {
			return nil, err
		}
		// we need to capture the returned value no matter the status
		// if the tx is reverted the error message is returned as returned value
		res.ReturnedData = execResult.ReturnData

		if !execResult.Failed() { // collect vm errors
			// If the transaction created a contract, store the creation address in the receipt,
			if msg.To == nil {
				deployedAddress := types.NewAddress(gethCrypto.CreateAddress(msg.From, msg.Nonce))
				res.DeployedContractAddress = &deployedAddress
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
	return &res, nil
}

func (proc *procedure) captureTraceBegin(
	depth int,
	typ gethVM.OpCode,
	from common.Address,
	to common.Address,
	input []byte,
	startGas uint64,
	value *big.Int) {
	tracer := proc.evm.Config.Tracer
	if tracer.OnTxStart != nil {
		tracer.OnTxStart(nil, gethTypes.NewTransaction(0, to, value, startGas, nil, input), from)
	}
	if tracer.OnEnter != nil {
		tracer.OnEnter(depth, byte(typ), from, to, input, startGas, value)
	}
	if tracer.OnGasChange != nil {
		tracer.OnGasChange(0, startGas, tracing.GasChangeCallInitialBalance)
	}
}

func (proc *procedure) captureTraceEnd(
	depth int,
	startGas uint64,
	ret []byte,
	receipt *gethTypes.Receipt,
	err error,
) {
	tracer := proc.evm.Config.Tracer
	leftOverGas := startGas - receipt.GasUsed
	if leftOverGas != 0 && tracer.OnGasChange != nil {
		tracer.OnGasChange(leftOverGas, 0, tracing.GasChangeCallLeftOverReturned)
	}
	var reverted bool
	if err != nil {
		reverted = true
	}
	if errors.Is(err, gethVM.ErrCodeStoreOutOfGas) {
		reverted = false
	}
	if tracer.OnExit != nil {
		tracer.OnExit(depth, ret, startGas-leftOverGas, gethVM.VMErrorFromErr(err), reverted)
	}
	if tracer.OnTxEnd != nil {
		tracer.OnTxEnd(receipt, err)
	}
}

func AddOne64th(n uint64) uint64 {
	// NOTE: Go's integer division floors, but that is desirable here
	return n + (n / 64)
}
