package emulator

import (
	"errors"
	"fmt"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethTracing "github.com/ethereum/go-ethereum/core/tracing"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/onflow/atree"
	"github.com/onflow/crypto/hash"

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
func (em *Emulator) NewReadOnlyBlockView() (types.ReadOnlyBlockView, error) {
	execState, err := state.NewStateDB(em.ledger, em.rootAddr)
	return &ReadOnlyBlockView{
		state: execState,
	}, err
}

// NewBlockView constructs a new block view (mutable)
func (em *Emulator) NewBlockView(ctx types.BlockContext) (types.BlockView, error) {
	return &BlockView{
		config:   newConfig(ctx),
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

	if !call.ValidEIP7825GasLimit(proc.config.ChainRules()) {
		res := &types.Result{
			TxType: call.Type,
			TxHash: call.Hash(),
		}
		res.SetValidationError(
			fmt.Errorf(
				"%w (cap: %d, tx: %d)",
				gethCore.ErrGasLimitTooHigh,
				gethParams.MaxTxGas,
				call.GasLimit,
			),
		)
		return res, nil
	}

	// Call tx tracer
	if proc.evm.Config.Tracer != nil && proc.evm.Config.Tracer.OnTxStart != nil {
		proc.evm.Config.Tracer.OnTxStart(proc.evm.GetVMContext(), call.Transaction(), call.From.ToCommon())
		defer func() {
			if proc.evm.Config.Tracer.OnTxEnd != nil &&
				err == nil && res != nil {
				proc.evm.Config.Tracer.OnTxEnd(res.Receipt(), res.ValidationError)
			}

			// call OnLog tracer hook, upon successful call result
			if proc.evm.Config.Tracer.OnLog != nil &&
				err == nil && res != nil {
				for _, log := range res.Logs {
					proc.evm.Config.Tracer.OnLog(log)
				}
			}
		}()
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
		return types.NewInvalidResult(tx.Type(), tx.Hash(), err), nil
	}

	// call tracer
	if proc.evm.Config.Tracer != nil && proc.evm.Config.Tracer.OnTxStart != nil {
		proc.evm.Config.Tracer.OnTxStart(proc.evm.GetVMContext(), tx, msg.From)
	}

	// run msg
	res, err := proc.run(msg, tx.Hash(), tx.Type())
	if err != nil {
		return nil, err
	}

	// all commit errors (StateDB errors) has to be returned
	res.StateChangeCommitment, err = proc.commit(true)
	if err != nil {
		return nil, err
	}

	// call tracer on tx end
	if proc.evm.Config.Tracer != nil &&
		proc.evm.Config.Tracer.OnTxEnd != nil &&
		res != nil {
		proc.evm.Config.Tracer.OnTxEnd(res.Receipt(), res.ValidationError)
	}

	// call OnLog tracer hook, upon successful tx result
	if proc.evm.Config.Tracer != nil &&
		proc.evm.Config.Tracer.OnLog != nil &&
		res != nil {
		for _, log := range res.Logs {
			proc.evm.Config.Tracer.OnLog(log)
		}
	}

	return res, nil
}

// BatchRunTransactions runs a batch of EVM transactions
func (bl *BlockView) BatchRunTransactions(txs []*gethTypes.Transaction) ([]*types.Result, error) {
	batchResults := make([]*types.Result, len(txs))

	// create a new procedure
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
			batchResults[i] = types.NewInvalidResult(tx.Type(), tx.Hash(), err)
			continue
		}

		// call tracer on tx start
		if proc.evm.Config.Tracer != nil && proc.evm.Config.Tracer.OnTxStart != nil {
			proc.evm.Config.Tracer.OnTxStart(proc.evm.GetVMContext(), tx, msg.From)
		}

		// run msg
		res, err := proc.run(msg, tx.Hash(), tx.Type())
		if err != nil {
			return nil, err
		}

		// all commit errors (StateDB errors) has to be returned
		res.StateChangeCommitment, err = proc.commit(false)
		if err != nil {
			return nil, err
		}

		// this clears state for any subsequent transaction runs
		proc.state.Reset()

		// collect result
		batchResults[i] = res

		// call tracer on tx end
		if proc.evm.Config.Tracer != nil &&
			proc.evm.Config.Tracer.OnTxEnd != nil &&
			res != nil {
			proc.evm.Config.Tracer.OnTxEnd(res.Receipt(), res.ValidationError)
		}

		// call OnLog tracer hook, upon successful tx result
		if proc.evm.Config.Tracer != nil &&
			proc.evm.Config.Tracer.OnLog != nil &&
			res != nil {
			for _, log := range res.Logs {
				proc.evm.Config.Tracer.OnLog(log)
			}
		}
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
	// create a new procedure
	proc, err := bl.newProcedure()
	if err != nil {
		return nil, err
	}

	// convert tx into message
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
	// we need to skip nonce/transaction checks for dry run
	msg.SkipNonceChecks = true
	msg.SkipTransactionChecks = true

	// run and return without committing the state changes
	return proc.run(msg, tx.Hash(), tx.Type())
}

func (bl *BlockView) newProcedure() (*procedure, error) {
	execState, err := state.NewStateDB(bl.ledger, bl.rootAddr)
	if err != nil {
		return nil, err
	}
	cfg := bl.config
	evm := gethVM.NewEVM(
		*cfg.BlockContext,
		execState,
		cfg.ChainConfig,
		cfg.EVMConfig,
	)
	evm.SetTxContext(*cfg.TxContext)
	// inject the applicable precompiled contracts for the current
	// chain rules, as well as any extra precompiled contracts,
	// such as Cadence Arch etc
	evm.SetPrecompiles(cfg.PrecompiledContracts)

	return &procedure{
		config: cfg,
		evm:    evm,
		state:  execState,
	}, nil
}

type procedure struct {
	config *Config
	evm    *gethVM.EVM
	state  types.StateDB
}

// commit commits the changes to the state (with optional finalization)
func (proc *procedure) commit(finalize bool) (hash.Hash, error) {
	// Calling `StateDB.Finalise(true)` is currently a no-op, but
	// we add it here to be more in line with how its envisioned.
	proc.state.Finalise(true)
	stateUpdateCommitment, err := proc.state.Commit(finalize)
	if err != nil {
		// if known types (state errors) don't do anything and return
		if types.IsAFatalError(err) || types.IsAStateError(err) || types.IsABackendError(err) {
			return stateUpdateCommitment, err
		}

		// else is a new fatal error
		return stateUpdateCommitment, types.NewFatalError(err)
	}
	return stateUpdateCommitment, nil
}

func (proc *procedure) mintTo(
	call *types.DirectCall,
) (*types.Result, error) {
	// check and convert value
	value, isValid := checkAndConvertValue(call.Value)
	if !isValid {
		return types.NewInvalidResult(
			call.Type,
			call.Hash(),
			types.ErrInvalidBalance,
		), nil
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
		// reset the state to revert the add balances
		proc.state.Reset()
		return res, nil
	}

	// commit and finalize the state and return any stateDB error
	res.StateChangeCommitment, err = proc.commit(true)
	return res, err
}

func (proc *procedure) withdrawFrom(
	call *types.DirectCall,
) (*types.Result, error) {
	// check and convert value
	value, isValid := checkAndConvertValue(call.Value)
	if !isValid {
		return types.NewInvalidResult(
			call.Type,
			call.Hash(),
			types.ErrInvalidBalance,
		), nil
	}

	// check balance is not prone to rounding error
	if !types.AttoFlowBalanceIsValidForFlowVault(call.Value) {
		return types.NewInvalidResult(
			call.Type,
			call.Hash(),
			types.ErrWithdrawBalanceRounding,
		), nil
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
		// reset the state to revert the add balances
		proc.state.Reset()
		return res, nil
	}

	// now deduct the balance from the bridge
	proc.state.SubBalance(bridge, value, gethTracing.BalanceIncreaseWithdrawal)

	// commit and finalize the state and return any stateDB error
	res.StateChangeCommitment, err = proc.commit(true)
	return res, err
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
	// check and convert value
	castedValue, isValid := checkAndConvertValue(call.Value)
	if !isValid {
		return types.NewInvalidResult(
			call.Type,
			call.Hash(),
			types.ErrInvalidBalance,
		), nil
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
	proc.state.SetNonce(
		callerCommon,
		proc.state.GetNonce(callerCommon)+1,
		gethTracing.NonceChangeContractCreator,
	)

	// setup account
	proc.state.CreateAccount(addr)
	proc.state.SetNonce(addr, 1, gethTracing.NonceChangeNewContract) // (EIP-158)
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
	contract := gethVM.NewContract(
		callerCommon,
		addr,
		castedValue,
		call.GasLimit,
		nil,
	)

	contract.SetCallCode(gethCrypto.Keccak256Hash(call.Data), call.Data)
	// update access list (Berlin)
	proc.state.AddAddressToAccessList(addr)

	ret, err := proc.evm.Run(contract, nil, false)
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

	proc.state.SetCode(addr, ret, gethTracing.CodeChangeContractCreation)
	res.StateChangeCommitment, err = proc.commit(true)
	return res, err
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
	res.StateChangeCommitment, err = proc.commit(true)
	return res, err
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
	execResult, err := gethCore.ApplyMessage(
		proc.evm,
		msg,
		gasPool,
	)
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
		res.MaxGasConsumed = execResult.MaxUsedGas
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
			// If the transaction has created a contract,
			// store the creation address in the receipt
			if msg.To == nil {
				deployedAddress := types.NewAddress(gethCrypto.CreateAddress(msg.From, msg.Nonce))
				res.DeployedContractAddress = &deployedAddress
			}
			// collect logs
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

func checkAndConvertValue(input *big.Int) (converted *uint256.Int, isValid bool) {
	// check for negative input
	if input.Sign() < 0 {
		return nil, false
	}
	// convert value into uint256
	value, overflow := uint256.FromBig(input)
	if overflow {
		return nil, false
	}
	return value, true
}
