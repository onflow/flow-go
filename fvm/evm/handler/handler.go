package handler

import (
	"bytes"
	"math/big"

	"github.com/onflow/cadence/runtime/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/rlp"

	"github.com/onflow/flow-go/fvm/environment"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/handler/coa"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const InvalidTransactionComputationCost = 1_000

// ContractHandler is responsible for triggering calls to emulator, metering,
// event emission and updating the block
type ContractHandler struct {
	flowChainID        flow.ChainID
	evmContractAddress flow.Address
	flowTokenAddress   common.Address
	blockStore         types.BlockStore
	addressAllocator   types.AddressAllocator
	backend            types.Backend
	emulator           types.Emulator
	precompiles        []types.Precompile
}

func (h *ContractHandler) FlowTokenAddress() common.Address {
	return h.flowTokenAddress
}

func (h *ContractHandler) EVMContractAddress() common.Address {
	return common.Address(h.evmContractAddress)
}

var _ types.ContractHandler = &ContractHandler{}

func NewContractHandler(
	flowChainID flow.ChainID,
	evmContractAddress flow.Address,
	flowTokenAddress common.Address,
	blockStore types.BlockStore,
	addressAllocator types.AddressAllocator,
	backend types.Backend,
	emulator types.Emulator,
) *ContractHandler {
	return &ContractHandler{
		flowChainID:        flowChainID,
		evmContractAddress: evmContractAddress,
		flowTokenAddress:   flowTokenAddress,
		blockStore:         blockStore,
		addressAllocator:   addressAllocator,
		backend:            backend,
		emulator:           emulator,
		precompiles:        preparePrecompiles(evmContractAddress, addressAllocator, backend),
	}
}

// DeployCOA deploys a cadence-owned-account and returns the address
func (h *ContractHandler) DeployCOA(uuid uint64) types.Address {
	addr, err := h.deployCOA(uuid)
	panicOnAnyError(err)
	return addr
}

func (h *ContractHandler) deployCOA(uuid uint64) (types.Address, error) {
	target := h.addressAllocator.AllocateCOAAddress(uuid)
	gaslimit := types.GasLimit(coa.ContractDeploymentRequiredGas)
	err := h.checkGasLimit(gaslimit)
	if err != nil {
		return types.Address{}, err
	}

	factory := h.addressAllocator.COAFactoryAddress()
	factoryAccount := h.AccountByAddress(factory, false)
	call := types.NewDeployCallWithTargetAddress(
		factory,
		target,
		coa.ContractBytes,
		uint64(gaslimit),
		new(big.Int),
		factoryAccount.Nonce(),
	)

	ctx, err := h.getBlockContext()
	if err != nil {
		return types.Address{}, err
	}
	res, err := h.executeAndHandleCall(ctx, call, nil, false)
	if err != nil {
		return types.Address{}, err
	}
	if res == nil || res.Failed() {
		return types.Address{}, types.ErrDirectCallExecutionFailed
	}

	return res.DeployedContractAddress, nil
}

// AccountByAddress returns the account for the given address,
// if isAuthorized is set, account is controlled by the FVM (COAs)
func (h *ContractHandler) AccountByAddress(addr types.Address, isAuthorized bool) types.Account {
	return newAccount(h, addr, isAuthorized)
}

// LastExecutedBlock returns the last executed block
func (h *ContractHandler) LastExecutedBlock() *types.Block {
	block, err := h.blockStore.LatestBlock()
	panicOnAnyError(err)
	return block
}

// RunOrPanic runs an rlpencoded evm transaction and
// collects the gas fees and pay it to the coinbase address provided.
func (h *ContractHandler) RunOrPanic(rlpEncodedTx []byte, coinbase types.Address) {
	_, err := h.run(rlpEncodedTx, coinbase)
	panicOnAnyError(err)
}

// Run tries to run an rlpencoded evm transaction and
// collects the gas fees and pay it to the coinbase address provided.
func (h *ContractHandler) Run(rlpEncodedTx []byte, coinbase types.Address) *types.ResultSummary {
	res, err := h.run(rlpEncodedTx, coinbase)
	panicOnFatalOrBackendError(err)
	return types.NewResultSummary(res, err)
}

func (h *ContractHandler) run(
	rlpEncodedTx []byte,
	coinbase types.Address,
) (*types.Result, error) {
	// step 1 - transaction decoding
	encodedLen := uint(len(rlpEncodedTx))
	err := h.backend.MeterComputation(environment.ComputationKindRLPDecoding, encodedLen)
	if err != nil {
		return nil, err
	}

	tx := gethTypes.Transaction{}
	err = tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(rlpEncodedTx),
			uint64(encodedLen)))
	if err != nil {
		return nil, types.NewEVMValidationError(err)
	}

	// step 2 - run transaction
	err = h.checkGasLimit(types.GasLimit(tx.Gas()))
	if err != nil {
		return nil, err
	}

	ctx, err := h.getBlockContext()
	if err != nil {
		return nil, err
	}
	ctx.GasFeeCollector = coinbase
	blk, err := h.emulator.NewBlockView(ctx)
	if err != nil {
		return nil, err
	}

	res, err := blk.RunTransaction(&tx)
	if err != nil {
		// if failed by validation errors
		// charge the InvalidTransactionComputationCost
		meterErr := h.chargeInvalidTxComputationCost()
		if meterErr != nil {
			return res, meterErr
		}
		return res, err
	}

	err = h.meterGasUsage(res)
	if err != nil {
		return res, err
	}

	// step 3 - update block proposal
	bp, err := h.blockStore.BlockProposal()
	if err != nil {
		return res, err
	}

	bp.AppendTxHash(res.TxHash)

	// Populate receipt root
	bp.PopulateReceiptRoot([]types.Result{*res})

	blockHash, err := bp.Hash()
	if err != nil {
		return res, err
	}

	// step 4 - emit events
	err = h.emitEvent(types.NewTransactionExecutedEvent(
		bp.Height,
		rlpEncodedTx,
		blockHash,
		res.TxHash,
		res,
	))
	if err != nil {
		return res, err
	}

	err = h.emitEvent(types.NewBlockExecutedEvent(bp))
	if err != nil {
		return res, err
	}

	// step 5 - commit block proposal
	return res, h.blockStore.CommitBlockProposal()
}

func (h *ContractHandler) checkGasLimit(limit types.GasLimit) error {
	// check gas limit against what has been left on the transaction side
	if !h.backend.ComputationAvailable(environment.ComputationKindEVMGasUsage, uint(limit)) {
		return types.ErrInsufficientComputation
	}
	return nil
}

func (h *ContractHandler) chargeInvalidTxComputationCost() error {
	return h.backend.MeterComputation(environment.ComputationKindEVMGasUsage, InvalidTransactionComputationCost)
}

func (h *ContractHandler) meterGasUsage(res *types.Result) error {
	if res != nil {
		return h.backend.MeterComputation(environment.ComputationKindEVMGasUsage, uint(res.GasConsumed))
	}
	return nil
}

func (h *ContractHandler) emitEvent(event *types.Event) error {
	ev, err := event.Payload.CadenceEvent()
	if err != nil {
		return err
	}
	return h.backend.EmitEvent(ev)
}

func (h *ContractHandler) getBlockContext() (types.BlockContext, error) {
	bp, err := h.blockStore.BlockProposal()
	if err != nil {
		return types.BlockContext{}, err
	}
	rand := gethCommon.Hash{}
	err = h.backend.ReadRandom(rand[:])
	if err != nil {
		return types.BlockContext{}, err
	}

	return types.BlockContext{
		ChainID:                types.EVMChainIDFromFlowChainID(h.flowChainID),
		BlockNumber:            bp.Height,
		DirectCallBaseGasUsage: types.DefaultDirectCallBaseGasUsage,
		GetHashFunc: func(n uint64) gethCommon.Hash {
			hash, err := h.blockStore.BlockHash(n)
			panicOnAnyError(err) // we have to handle it here given we can't continue with it even in try case
			return hash
		},
		ExtraPrecompiles: h.precompiles,
		Random:           rand,
	}, nil
}

func (h *ContractHandler) executeAndHandleCall(
	ctx types.BlockContext,
	call *types.DirectCall,
	totalSupplyDiff *big.Int,
	deductSupplyDiff bool,
) (*types.Result, error) {
	var res *types.Result
	// execute the call
	blk, err := h.emulator.NewBlockView(ctx)
	if err != nil {
		return res, err
	}

	res, err = blk.DirectCall(call)
	if err != nil {
		// if failed by validation errors
		// charge the InvalidTransactionComputationCost
		meterErr := h.chargeInvalidTxComputationCost()
		if meterErr != nil {
			return res, meterErr
		}
		return res, err
	}

	err = h.meterGasUsage(res)
	if err != nil {
		return res, err
	}

	// update block proposal
	bp, err := h.blockStore.BlockProposal()
	if err != nil {
		return res, err
	}

	bp.AppendTxHash(res.TxHash)

	// Populate receipt root
	bp.PopulateReceiptRoot([]types.Result{*res})

	if totalSupplyDiff != nil {
		if deductSupplyDiff {
			bp.TotalSupply = new(big.Int).Sub(bp.TotalSupply, totalSupplyDiff)
			if bp.TotalSupply.Sign() < 0 {
				return res, types.ErrInsufficientTotalSupply
			}
		} else {
			bp.TotalSupply = new(big.Int).Add(bp.TotalSupply, totalSupplyDiff)
		}
	}

	blockHash, err := bp.Hash()
	if err != nil {
		return res, err
	}

	// emit events
	encoded, err := call.Encode()
	if err != nil {
		return res, err
	}

	err = h.emitEvent(
		types.NewTransactionExecutedEvent(
			bp.Height,
			encoded,
			blockHash,
			res.TxHash,
			res,
		),
	)
	if err != nil {
		return res, err
	}

	err = h.emitEvent(types.NewBlockExecutedEvent(bp))
	if err != nil {
		return res, err
	}

	// commit block proposal
	return res, h.blockStore.CommitBlockProposal()
}

func (h *ContractHandler) GenerateResourceUUID() uint64 {
	uuid, err := h.backend.GenerateUUID()
	panicOnAnyError(err)
	return uuid
}

type Account struct {
	isAuthorized bool
	address      types.Address
	fch          *ContractHandler
}

// newAccount creates a new evm account
func newAccount(fch *ContractHandler, addr types.Address, isAuthorized bool) *Account {
	return &Account{
		isAuthorized: isAuthorized,
		fch:          fch,
		address:      addr,
	}
}

// Address returns the address associated with the account
func (a *Account) Address() types.Address {
	return a.address
}

// Nonce returns the nonce of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already transalates into computation
func (a *Account) Nonce() uint64 {
	nonce, err := a.nonce()
	panicOnAnyError(err)
	return nonce
}

func (a *Account) nonce() (uint64, error) {
	ctx, err := a.fch.getBlockContext()
	if err != nil {
		return 0, err
	}

	blk, err := a.fch.emulator.NewReadOnlyBlockView(ctx)
	if err != nil {
		return 0, err
	}

	return blk.NonceOf(a.address)
}

// Balance returns the balance of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already transalates into computation
func (a *Account) Balance() types.Balance {
	bal, err := a.balance()
	panicOnAnyError(err)
	return bal
}

func (a *Account) balance() (types.Balance, error) {
	ctx, err := a.fch.getBlockContext()
	if err != nil {
		return nil, err
	}

	blk, err := a.fch.emulator.NewReadOnlyBlockView(ctx)
	if err != nil {
		return nil, err
	}

	bl, err := blk.BalanceOf(a.address)
	return types.NewBalance(bl), err
}

// Code returns the code of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already transalates into computation
func (a *Account) Code() types.Code {
	code, err := a.code()
	panicOnAnyError(err)
	return code
}

func (a *Account) code() (types.Code, error) {
	ctx, err := a.fch.getBlockContext()
	if err != nil {
		return nil, err
	}

	blk, err := a.fch.emulator.NewReadOnlyBlockView(ctx)
	if err != nil {
		return nil, err
	}
	return blk.CodeOf(a.address)
}

// CodeHash returns the code hash of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already transalates into computation
func (a *Account) CodeHash() []byte {
	codeHash, err := a.codeHash()
	panicOnAnyError(err)
	return codeHash
}

func (a *Account) codeHash() ([]byte, error) {
	ctx, err := a.fch.getBlockContext()
	if err != nil {
		return nil, err
	}

	blk, err := a.fch.emulator.NewReadOnlyBlockView(ctx)
	if err != nil {
		return nil, err
	}
	return blk.CodeHashOf(a.address)
}

// Deposit deposits the token from the given vault into the flow evm main vault
// and update the account balance with the new amount
func (a *Account) Deposit(v *types.FLOWTokenVault) {
	err := a.deposit(v)
	panicOnAnyError(err)
}

func (a *Account) deposit(v *types.FLOWTokenVault) error {
	bridge := a.fch.addressAllocator.NativeTokenBridgeAddress()
	bridgeAccount := a.fch.AccountByAddress(bridge, false)

	call := types.NewDepositCall(
		bridge,
		a.address,
		v.Balance(),
		bridgeAccount.Nonce(),
	)
	ctx, err := a.precheck(false, types.GasLimit(call.GasLimit))
	if err != nil {
		return err
	}

	res, err := a.fch.executeAndHandleCall(ctx, call, v.Balance(), false)
	if err != nil {
		return err
	}

	if res == nil || res.Failed() {
		return types.ErrDirectCallExecutionFailed
	}

	return nil
}

// Withdraw deducts the balance from the account and
// withdraw and return flow token from the Flex main vault.
func (a *Account) Withdraw(b types.Balance) *types.FLOWTokenVault {
	v, err := a.withdraw(b)
	panicOnAnyError(err)
	return v
}

func (a *Account) withdraw(b types.Balance) (*types.FLOWTokenVault, error) {
	call := types.NewWithdrawCall(
		a.fch.addressAllocator.NativeTokenBridgeAddress(),
		a.address,
		b,
		a.Nonce(),
	)

	ctx, err := a.precheck(true, types.GasLimit(call.GasLimit))
	if err != nil {
		return nil, err
	}

	// Don't allow withdraw for balances that has rounding error
	if types.BalanceConvertionToUFix64ProneToRoundingError(b) {
		return nil, types.ErrWithdrawBalanceRounding
	}

	res, err := a.fch.executeAndHandleCall(ctx, call, b, true)
	if err != nil {
		return nil, err
	}

	if res == nil || res.Failed() {
		return nil, types.ErrDirectCallExecutionFailed
	}

	return types.NewFlowTokenVault(b), nil
}

// Transfer transfers tokens between accounts
func (a *Account) Transfer(to types.Address, balance types.Balance) {
	err := a.transfer(to, balance)
	panicOnAnyError(err)
}

func (a *Account) transfer(to types.Address, balance types.Balance) error {
	call := types.NewTransferCall(
		a.address,
		to,
		balance,
		a.Nonce(),
	)
	ctx, err := a.precheck(true, types.GasLimit(call.GasLimit))
	if err != nil {
		return err
	}
	res, err := a.fch.executeAndHandleCall(ctx, call, nil, false)
	if err != nil {
		return err
	}

	if res == nil || res.Failed() {
		return types.ErrDirectCallExecutionFailed
	}

	return nil
}

// Deploy deploys a contract to the EVM environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the caller accounts
func (a *Account) Deploy(code types.Code, gaslimit types.GasLimit, balance types.Balance) types.Address {
	addr, err := a.deploy(code, gaslimit, balance)
	panicOnAnyError(err)
	return addr
}

func (a *Account) deploy(code types.Code, gaslimit types.GasLimit, balance types.Balance) (types.Address, error) {
	ctx, err := a.precheck(true, gaslimit)
	if err != nil {
		return types.Address{}, err
	}

	call := types.NewDeployCall(
		a.address,
		code,
		uint64(gaslimit),
		balance,
		a.Nonce(),
	)
	res, err := a.fch.executeAndHandleCall(ctx, call, nil, false)
	if err != nil {
		return types.Address{}, err
	}

	if res == nil || res.Failed() {
		return types.Address{}, types.ErrDirectCallExecutionFailed
	}

	return types.Address(res.DeployedContractAddress), nil
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
func (a *Account) Call(to types.Address, data types.Data, gaslimit types.GasLimit, balance types.Balance) *types.ResultSummary {
	res, err := a.call(to, data, gaslimit, balance)
	panicOnFatalOrBackendError(err)
	return types.NewResultSummary(res, err)
}

func (a *Account) call(to types.Address, data types.Data, gaslimit types.GasLimit, balance types.Balance) (*types.Result, error) {
	ctx, err := a.precheck(true, gaslimit)
	if err != nil {
		return nil, err
	}
	call := types.NewContractCall(
		a.address,
		to,
		data,
		uint64(gaslimit),
		balance,
		a.Nonce(),
	)

	return a.fch.executeAndHandleCall(ctx, call, nil, false)
}

func (a *Account) precheck(authroized bool, gaslimit types.GasLimit) (types.BlockContext, error) {
	// check if account is authorized (i.e. is a COA)
	if authroized && !a.isAuthorized {
		return types.BlockContext{}, types.ErrUnAuthroizedMethodCall
	}
	err := a.fch.checkGasLimit(gaslimit)
	if err != nil {
		return types.BlockContext{}, err
	}

	return a.fch.getBlockContext()
}

func panicOnAnyError(err error) {
	if err == nil {
		return
	}

	panicOnFatalOrBackendError(err)

	// if not FVM wrap it with EVM error and panic
	panic(fvmErrors.NewEVMError(err))
}

// panicOnFatalOrBackendError errors panic on fatal or backend-related errors
func panicOnFatalOrBackendError(err error) {
	if err == nil {
		return
	}

	if types.IsAFatalError(err) {
		// don't wrap it
		panic(fvmErrors.NewEVMFailure(err))
	}

	if types.IsABackendError(err) {
		// backend errors doesn't need wrapping
		panic(err)
	}
}
