package emulator

import (
	"errors"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/onflow/atree"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	gethTracing "github.com/onflow/go-ethereum/core/tracing"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	gethParams "github.com/onflow/go-ethereum/params"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// Emulator wraps an EVM runtime where evm transactions
// and direct calls are accepted.
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
		WithBlockTotalGasUsedSoFar(ctx.TotalGasUsedSoFar),
		WithBlockTxCountSoFar(ctx.TxCountSoFar),
	)
}

// NewReadOnlyBlockView constructs a new read-only block view
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
// could be used for multiple queries against a block
type ReadOnlyBlockView struct {
	state types.StateDB
}

// BalanceOf returns the balance of the given address
func (bv *ReadOnlyBlockView) BalanceOf(address types.Address) (*big.Int, error) {
	bal := bv.state.GetBalance(address.ToCommon())
	return bal.ToBig(), bv.state.Error()
}

// NonceOf returns the nonce of the given address
func (bv *ReadOnlyBlockView) NonceOf(address types.Address) (uint64, error) {
	return bv.state.GetNonce(address.ToCommon()), bv.state.Error()
}

// CodeOf returns the code of the given address
func (bv *ReadOnlyBlockView) CodeOf(address types.Address) (types.Code, error) {
	return bv.state.GetCode(address.ToCommon()), bv.state.Error()
}

// CodeHashOf returns the code hash of the given address
func (bv *ReadOnlyBlockView) CodeHashOf(address types.Address) ([]byte, error) {
	return bv.state.GetCodeHash(address.ToCommon()).Bytes(), bv.state.Error()
}

// BlockView allows mutation of the evm state as part of a block
// current version only accepts only a single interaction per block view.
type BlockView struct {
	config   *Config
	rootAddr flow.Address
	ledger   atree.Ledger
}

// DirectCall executes a direct call
func (bl *BlockView) DirectCall(call *types.DirectCall) (res *types.Result, err error) {
	// construct a new procedure
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	// Set the nonce for the call (needed for some operations like deployment)
	call.Nonce = proc.state.GetNonce(call.From.ToCommon())

	// Call tx tracer
	if proc.evm.Config.Tracer != nil && proc.evm.Config.Tracer.OnTxStart != nil {
		proc.evm.Config.Tracer.OnTxStart(proc.evm.GetVMContext(), call.Transaction(), call.From.ToCommon())
		if proc.evm.Config.Tracer.OnTxEnd != nil {
			defer func() {
				proc.evm.Config.Tracer.OnTxEnd(res.Receipt(), err)
			}()
		}
	}

	// re-route based on the sub type
	switch call.SubType {
	case types.DepositCallSubType:
		return proc.mintTo(call)
	case types.WithdrawCallSubType:
		return proc.withdrawFrom(call)
	case types.DeployCallSubType:
		if !call.EmptyToField() {
			return proc.deployAt(call)
		}
		fallthrough
	default:
		return proc.runDirect(call)
	}
}

// RunTransaction runs an evm transaction
func (bl *BlockView) RunTransaction(
	tx *gethTypes.Transaction,
) (result *types.Result, err error) {
	// create a new procedure
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	// constructs a core.message from the tx
	msg, err := gethCore.TransactionToMessage(
		tx,
		GetSigner(bl.config),
		proc.config.BlockContext.BaseFee)
	if err != nil {
		// this is not a fatal error (e.g. due to bad signature)
		// not a valid transaction
		return types.NewInvalidResult(tx, err), nil
	}

	// call tracer
	if proc.evm.Config.Tracer != nil && proc.evm.Config.Tracer.OnTxStart != nil {
		proc.evm.Config.Tracer.OnTxStart(proc.evm.GetVMContext(), tx, msg.From)
		if proc.evm.Config.Tracer.OnTxEnd != nil {
			defer func() {
				proc.evm.Config.Tracer.OnTxEnd(result.Receipt(), err)
			}()
		}
	}

	// run msg
	res, err := proc.run(msg, tx.Hash(), tx.Type())
	if err != nil {
		return nil, err
	}
	// all commit errors (StateDB errors) has to be returned
	if err := proc.commit(true); err != nil {
		return nil, err
	}

	return res, nil
}

// BatchRunTransactions runs a batch of EVM transactions
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

		// call tracer
		if proc.evm.Config.Tracer != nil && proc.evm.Config.Tracer.OnTxStart != nil {
			proc.evm.Config.Tracer.OnTxStart(proc.evm.GetVMContext(), tx, msg.From)
			if proc.evm.Config.Tracer.OnTxEnd != nil {
				defer func() {
					proc.evm.Config.Tracer.OnTxEnd(
						batchResults[i].Receipt(),
						err,
					)
				}()
			}
		}

		// run msg
		res, err := proc.run(msg, tx.Hash(), tx.Type())
		if err != nil {
			return nil, err
		}
		// all commit errors (StateDB errors) has to be returned
		if err := proc.commit(false); err != nil {
			return nil, err
		}

		// this clears state for any subsequent transaction runs
		proc.state.Reset()
		// collect result
		batchResults[i] = res
	}

	// finalize after all the batch transactions are executed to save resources
	if err := proc.state.Finalize(); err != nil {
		return nil, err
	}

	return batchResults, nil
}

// DryRunTransaction runs an unsigned transaction without persisting the state
func (bl *BlockView) DryRunTransaction(
	tx *gethTypes.Transaction,
	from gethCommon.Address,
) (*types.Result, error) {
	var txResult *types.Result
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}
	msg, err := gethCore.TransactionToMessage(
		tx,
		GetSigner(bl.config),
		proc.config.BlockContext.BaseFee,
	)

	// we can ignore invalid signature errors since we don't expect signed transactions
	if !errors.Is(err, gethTypes.ErrInvalidSig) {
		return nil, err
	}

	// use the from as the signer
	msg.From = from
	// we need to skip nonce check for dry run
	msg.SkipAccountChecks = true

	// return without committing the state
	txResult, err = proc.run(msg, tx.Hash(), tx.Type())
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
		if types.IsAFatalError(err) || types.IsAStateError(err) || types.IsABackendError(err) {
			return err
		}

		// else is a new fatal error
		return types.NewFatalError(err)
	}
	return nil
}

func (proc *procedure) mintTo(
	call *types.DirectCall,
) (*types.Result, error) {
	// convert and check value
	isValid, value := convertAndCheckValue(call.Value)
	if !isValid {
		return types.NewInvalidResult(
			call.Transaction(),
			types.ErrInvalidBalance), nil
	}

	// create bridge account if not exist
	bridge := call.From.ToCommon()
	if !proc.state.Exist(bridge) {
		proc.state.CreateAccount(bridge)
	}

	// add balance to the bridge account before transfer
	proc.state.AddBalance(bridge, value, gethTracing.BalanceIncreaseWithdrawal)
	// check state errors
	if err := proc.state.Error(); err != nil {
		return nil, err
	}

	// withdraw the amount and move it to the bridge account
	res, err := proc.run(call.Message(), call.Hash(), types.DirectCallTxType)
	if err != nil {
		return res, err
	}

	// if any error (invalid or vm) on the internal call, revert and don't commit any change
	// this prevents having cases that we add balance to the bridge but the transfer
	// fails due to gas, etc.
	if res.Invalid() || res.Failed() {
		return &types.Result{
			TxType:          call.Type,
			GasConsumed:     types.InvalidTransactionGasCost,
			ValidationError: types.ErrInternalDirectCallFailed,
		}, nil
	}

	// commit and finalize the state and return any stateDB error
	return res, proc.commit(true)
}

func (proc *procedure) withdrawFrom(
	call *types.DirectCall,
) (*types.Result, error) {
	// convert and check value
	isValid, value := convertAndCheckValue(call.Value)
	if !isValid {
		return types.NewInvalidResult(
			call.Transaction(),
			types.ErrInvalidBalance), nil
	}

	// create bridge account if not exist
	bridge := call.To.ToCommon()
	if !proc.state.Exist(bridge) {
		proc.state.CreateAccount(bridge)
	}

	// withdraw the amount and move it to the bridge account
	res, err := proc.run(call.Message(), call.Hash(), types.DirectCallTxType)
	if err != nil {
		return res, err
	}

	// if any error (invalid or vm) on the internal call, revert and don't commit any change
	// this prevents having cases that we deduct the balance from the account
	// but doesn't return it as a vault.
	if res.Invalid() || res.Failed() {
		return &types.Result{
			TxType:          call.Type,
			GasConsumed:     types.InvalidTransactionGasCost,
			ValidationError: types.ErrInternalDirectCallFailed,
		}, nil
	}

	// now deduct the balance from the bridge
	proc.state.SubBalance(bridge, value, gethTracing.BalanceIncreaseWithdrawal)

	// commit and finalize the state and return any stateDB error
	return res, proc.commit(true)
}

// deployAt deploys a contract at the given target address
// behavior should be similar to what evm.create internal method does with
// a few differences, we don't need to check for previous forks given this
// functionality was not available to anyone, we don't need to
// follow snapshoting, given we do commit/revert style in this code base.
// in the future we might optimize this method accepting deploy-ready byte codes
// and skip interpreter call, gas calculations and many checks.
func (proc *procedure) deployAt(
	call *types.DirectCall,
) (*types.Result, error) {
	// convert and check value
	isValid, castedValue := convertAndCheckValue(call.Value)
	if !isValid {
		return types.NewInvalidResult(
			call.Transaction(),
			types.ErrInvalidBalance), nil
	}

	txHash := call.Hash()
	res := &types.Result{
		TxType: types.DirectCallTxType,
		TxHash: txHash,
	}

	if proc.evm.Config.Tracer != nil {
		tracer := proc.evm.Config.Tracer
		if tracer.OnEnter != nil {
			tracer.OnEnter(0, byte(gethVM.CREATE2), call.From.ToCommon(), call.To.ToCommon(), call.Data, call.GasLimit, call.Value)
		}
		if tracer.OnGasChange != nil {
			tracer.OnGasChange(0, call.GasLimit, gethTracing.GasChangeCallInitialBalance)
		}

		defer func() {
			if call.GasLimit != 0 && tracer.OnGasChange != nil {
				tracer.OnGasChange(call.GasLimit, 0, gethTracing.GasChangeCallLeftOverReturned)
			}
			if tracer.OnExit != nil {
				var reverted bool
				if res.VMError != nil && !errors.Is(res.VMError, gethVM.ErrCodeStoreOutOfGas) {
					reverted = true
				}
				tracer.OnExit(0, res.ReturnedData, call.GasLimit-res.GasConsumed, gethVM.VMErrorFromErr(res.VMError), reverted)
			}
		}()
	}

	addr := call.To.ToCommon()
	// pre check 1 - check balance of the source
	if call.Value.Sign() != 0 &&
		!proc.evm.Context.CanTransfer(proc.state, call.From.ToCommon(), castedValue) {
		res.SetValidationError(gethCore.ErrInsufficientFundsForTransfer)
		return res, nil
	}

	// pre check 2 - ensure there's no existing eoa or contract is deployed at the address
	contractHash := proc.state.GetCodeHash(addr)
	if proc.state.GetNonce(addr) != 0 ||
		(contractHash != (gethCommon.Hash{}) && contractHash != gethTypes.EmptyCodeHash) {
		res.VMError = gethVM.ErrContractAddressCollision
		return res, nil
	}

	callerCommon := call.From.ToCommon()
	// setup caller if doesn't exist
	if !proc.state.Exist(callerCommon) {
		proc.state.CreateAccount(callerCommon)
	}
	// increment the nonce for the caller
	proc.state.SetNonce(callerCommon, proc.state.GetNonce(callerCommon)+1)

	// setup account
	proc.state.CreateAccount(addr)
	proc.state.SetNonce(addr, 1) // (EIP-158)
	if call.Value.Sign() > 0 {
		proc.evm.Context.Transfer( // transfer value
			proc.state,
			callerCommon,
			addr,
			uint256.MustFromBig(call.Value),
		)
	}

	// run code through interpreter
	// this would check for errors and computes the final bytes to be stored under account
	var err error
	inter := gethVM.NewEVMInterpreter(proc.evm)
	contract := gethVM.NewContract(
		gethVM.AccountRef(callerCommon),
		gethVM.AccountRef(addr),
		castedValue,
		call.GasLimit)

	contract.SetCallCode(&addr, gethCrypto.Keccak256Hash(call.Data), call.Data)
	// update access list (Berlin)
	proc.state.AddAddressToAccessList(addr)

	ret, err := inter.Run(contract, nil, false)
	gasCost := uint64(len(ret)) * gethParams.CreateDataGas
	res.GasConsumed = gasCost

	// handle errors
	if err != nil {
		// for all errors except this one consume all the remaining gas (Homestead)
		if err != gethVM.ErrExecutionReverted {
			res.GasConsumed = call.GasLimit
		}
		res.VMError = err
		return res, nil
	}

	// update gas usage
	if gasCost > call.GasLimit {
		// consume all the remaining gas (Homestead)
		res.GasConsumed = call.GasLimit
		res.VMError = gethVM.ErrCodeStoreOutOfGas
		return res, nil
	}

	// check max code size (EIP-158)
	if len(ret) > gethParams.MaxCodeSize {
		// consume all the remaining gas (Homestead)
		res.GasConsumed = call.GasLimit
		res.VMError = gethVM.ErrMaxCodeSizeExceeded
		return res, nil
	}

	// reject code starting with 0xEF (EIP-3541)
	if len(ret) >= 1 && ret[0] == 0xEF {
		// consume all the remaining gas (Homestead)
		res.GasConsumed = call.GasLimit
		res.VMError = gethVM.ErrInvalidCode
		return res, nil
	}

	res.DeployedContractAddress = &call.To
	res.CumulativeGasUsed = proc.config.BlockTotalGasUsedSoFar + res.GasConsumed

	proc.state.SetCode(addr, ret)
	return res, proc.commit(true)
}

func (proc *procedure) runDirect(
	call *types.DirectCall,
) (*types.Result, error) {
	// run the msg
	res, err := proc.run(
		call.Message(),
		call.Hash(),
		types.DirectCallTxType,
	)
	if err != nil {
		return nil, err
	}
	// commit and finalize the state and return any stateDB error
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
	txType uint8,
) (*types.Result, error) {
	var err error
	res := types.Result{
		TxType: txType,
		TxHash: txHash,
	}

	// Negative values are not acceptable
	// although we check this condition on direct calls
	// its worth an extra check here given some calls are
	// coming from batch run, etc.
	if msg.Value.Sign() < 0 {
		res.SetValidationError(types.ErrInvalidBalance)
		return &res, nil
	}

	// set the origin on the TxContext
	proc.evm.TxContext.Origin = msg.From

	// reset precompile tracking in case
	proc.config.PCTracker.Reset()

	// Set gas pool based on block gas limit
	// if the block gas limit is set to anything than max
	// we need to update this code.
	gasPool := (*gethCore.GasPool)(&proc.config.BlockContext.GasLimit)
	// transit the state
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

	txIndex := proc.config.BlockTxCountSoFar
	// if pre-checks are passed, the exec result won't be nil
	if execResult != nil {

		res.GasConsumed = execResult.UsedGas
		res.GasRefund = proc.state.GetRefund()
		res.Index = uint16(txIndex)
		res.CumulativeGasUsed = execResult.UsedGas + proc.config.BlockTotalGasUsedSoFar
		res.PrecompiledCalls, err = proc.config.PCTracker.CapturedCalls()
		if err != nil {
			return nil, err
		}

		// we need to capture the returned value no matter the status
		// if the tx is reverted the error message is returned as returned value
		res.ReturnedData = execResult.ReturnData

		// Update proc context
		proc.config.BlockTotalGasUsedSoFar = res.CumulativeGasUsed
		proc.config.BlockTxCountSoFar += 1

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

func AddOne64th(n uint64) uint64 {
	// NOTE: Go's integer division floors, but that is desirable here
	return n + (n / 64)
}

func convertAndCheckValue(input *big.Int) (isValid bool, converted *uint256.Int) {
	// check for negative input
	if input.Sign() < 0 {
		return false, nil
	}
	// convert value into uint256
	value, overflow := uint256.FromBig(input)
	if overflow {
		return true, nil
	}
	return true, value
}
