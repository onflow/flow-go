package handler

import (
	"bytes"
	"math/big"

	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/types"
)

// ContractHandler is responsible for triggering calls to emulator, metering,
// event emission and updating the block
type ContractHandler struct {
	flowTokenAddress common.Address
	blockstore       types.BlockStore
	addressAllocator types.AddressAllocator
	backend          types.Backend
	emulator         types.Emulator
	precompiles      []types.Precompile
}

func (h *ContractHandler) FlowTokenAddress() common.Address {
	return h.flowTokenAddress
}

var _ types.ContractHandler = &ContractHandler{}

func NewContractHandler(
	flowTokenAddress common.Address,
	blockstore types.BlockStore,
	addressAllocator types.AddressAllocator,
	backend types.Backend,
	emulator types.Emulator,
) *ContractHandler {
	return &ContractHandler{
		flowTokenAddress: flowTokenAddress,
		blockstore:       blockstore,
		addressAllocator: addressAllocator,
		backend:          backend,
		emulator:         emulator,
		precompiles:      getPrecompiles(addressAllocator, backend),
	}
}

func getPrecompiles(
	addressAllocator types.AddressAllocator,
	backend types.Backend,
) []types.Precompile {
	archAddress := addressAllocator.AllocatePrecompileAddress(1)
	archContract := precompiles.ArchContract(
		archAddress,
		backend.GetCurrentBlockHeight,
	)
	return []types.Precompile{archContract}
}

// AllocateAddress allocates an address to be used by the bridged accounts
func (h *ContractHandler) AllocateAddress() types.Address {
	target, err := h.addressAllocator.AllocateCOAAddress()
	handleError(err)
	return target
}

// AccountByAddress returns the account for the given address,
// if isAuthorized is set, account is controlled by the FVM (bridged accounts)
func (h *ContractHandler) AccountByAddress(addr types.Address, isAuthorized bool) types.Account {
	return newAccount(h, addr, isAuthorized)
}

// LastExecutedBlock returns the last executed block
func (h *ContractHandler) LastExecutedBlock() *types.Block {
	block, err := h.blockstore.LatestBlock()
	handleError(err)
	return block
}

// Run runs an rlpencoded evm transaction and
// collects the gas fees and pay it to the coinbase address provided.
func (h *ContractHandler) Run(rlpEncodedTx []byte, coinbase types.Address) {
	// step 1 - transaction decoding
	encodedLen := uint(len(rlpEncodedTx))
	err := h.backend.MeterComputation(environment.ComputationKindRLPDecoding, encodedLen)
	handleError(err)

	tx := gethTypes.Transaction{}
	err = tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(rlpEncodedTx),
			uint64(encodedLen)))
	handleError(err)

	// step 2 - run transaction
	h.checkGasLimit(types.GasLimit(tx.Gas()))

	ctx := h.getBlockContext()
	ctx.GasFeeCollector = coinbase
	blk, err := h.emulator.NewBlockView(ctx)
	handleError(err)

	res, err := blk.RunTransaction(&tx)
	h.meterGasUsage(res)
	handleError(err)

	// step 3 - update block proposal
	bp, err := h.blockstore.BlockProposal()
	handleError(err)

	txHash := tx.Hash()
	bp.AppendTxHash(txHash)

	// step 4 - emit events
	h.emitEvent(types.NewTransactionExecutedEvent(
		bp.Height,
		rlpEncodedTx,
		txHash,
		res,
	))
	h.emitEvent(types.NewBlockExecutedEvent(bp))

	// step 5 - commit block proposal
	err = h.blockstore.CommitBlockProposal()
	handleError(err)
}

func (h *ContractHandler) checkGasLimit(limit types.GasLimit) {
	// check gas limit against what has been left on the transaction side
	if !h.backend.ComputationAvailable(environment.ComputationKindEVMGasUsage, uint(limit)) {
		handleError(types.ErrInsufficientComputation)
	}
}

func (h *ContractHandler) meterGasUsage(res *types.Result) {
	if res != nil {
		err := h.backend.MeterComputation(environment.ComputationKindEVMGasUsage, uint(res.GasConsumed))
		handleError(err)
	}
}

func (h *ContractHandler) emitEvent(event *types.Event) {
	ev, err := event.Payload.CadenceEvent()
	handleError(err)

	err = h.backend.EmitEvent(ev)
	handleError(err)
}

func (h *ContractHandler) getBlockContext() types.BlockContext {
	bp, err := h.blockstore.BlockProposal()
	handleError(err)
	return types.BlockContext{
		BlockNumber:            bp.Height,
		DirectCallBaseGasUsage: types.DefaultDirectCallBaseGasUsage,
		ExtraPrecompiles:       h.precompiles,
	}
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

// Address returns the address associated with the bridged account
func (a *Account) Address() types.Address {
	return a.address
}

// Balance returns the balance of this bridged account
//
// TODO: we might need to meter computation for read only operations as well
// currently the storage limits is enforced
func (a *Account) Balance() types.Balance {
	ctx := a.fch.getBlockContext()

	blk, err := a.fch.emulator.NewReadOnlyBlockView(ctx)
	handleError(err)

	bl, err := blk.BalanceOf(a.address)
	handleError(err)

	balance, err := types.NewBalanceFromAttoFlow(bl)
	handleError(err)
	return balance
}

// Deposit deposits the token from the given vault into the flow evm main vault
// and update the account balance with the new amount
func (a *Account) Deposit(v *types.FLOWTokenVault) {
	cfg := a.fch.getBlockContext()
	a.fch.checkGasLimit(types.GasLimit(cfg.DirectCallBaseGasUsage))

	call := types.NewDepositCall(
		a.address,
		v.Balance().ToAttoFlow(),
	)
	a.executeAndHandleCall(a.fch.getBlockContext(), call, v.Balance().ToAttoFlow(), false)
}

// Withdraw deducts the balance from the account and
// withdraw and return flow token from the Flex main vault.
func (a *Account) Withdraw(b types.Balance) *types.FLOWTokenVault {
	a.checkAuthorized()

	cfg := a.fch.getBlockContext()
	a.fch.checkGasLimit(types.GasLimit(cfg.DirectCallBaseGasUsage))

	// check balance of flex vault
	bp, err := a.fch.blockstore.BlockProposal()
	handleError(err)
	// b > total supply
	if b.ToAttoFlow().Cmp(bp.TotalSupply) == 1 {
		handleError(types.ErrInsufficientTotalSupply)
	}

	call := types.NewWithdrawCall(
		a.address,
		b.ToAttoFlow(),
	)
	a.executeAndHandleCall(a.fch.getBlockContext(), call, b.ToAttoFlow(), true)

	return types.NewFlowTokenVault(b)
}

// Transfer transfers tokens between accounts
func (a *Account) Transfer(to types.Address, balance types.Balance) {
	a.checkAuthorized()

	ctx := a.fch.getBlockContext()
	a.fch.checkGasLimit(types.GasLimit(ctx.DirectCallBaseGasUsage))

	call := types.NewTransferCall(
		a.address,
		to,
		balance.ToAttoFlow(),
	)
	a.executeAndHandleCall(ctx, call, nil, false)
}

// Deploy deploys a contract to the EVM environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the caller accounts
func (a *Account) Deploy(code types.Code, gaslimit types.GasLimit, balance types.Balance) types.Address {
	a.checkAuthorized()
	a.fch.checkGasLimit(gaslimit)

	call := types.NewDeployCall(
		a.address,
		code,
		uint64(gaslimit),
		balance.ToAttoFlow(),
	)
	res := a.executeAndHandleCall(a.fch.getBlockContext(), call, nil, false)
	return types.Address(res.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
func (a *Account) Call(to types.Address, data types.Data, gaslimit types.GasLimit, balance types.Balance) types.Data {
	a.checkAuthorized()
	a.fch.checkGasLimit(gaslimit)
	call := types.NewContractCall(
		a.address,
		to,
		data,
		uint64(gaslimit),
		balance.ToAttoFlow(),
	)
	res := a.executeAndHandleCall(a.fch.getBlockContext(), call, nil, false)
	return res.ReturnedValue
}

func (a *Account) executeAndHandleCall(
	ctx types.BlockContext,
	call *types.DirectCall,
	totalSupplyDiff *big.Int,
	deductSupplyDiff bool,
) *types.Result {
	// execute the call
	blk, err := a.fch.emulator.NewBlockView(ctx)
	handleError(err)

	res, err := blk.DirectCall(call)
	a.fch.meterGasUsage(res)
	handleError(err)

	// update block proposal
	callHash, err := call.Hash()
	if err != nil {
		err = types.NewFatalError(err)
		handleError(err)
	}

	bp, err := a.fch.blockstore.BlockProposal()
	handleError(err)
	bp.AppendTxHash(callHash)
	if totalSupplyDiff != nil {
		if deductSupplyDiff {
			bp.TotalSupply = new(big.Int).Sub(bp.TotalSupply, totalSupplyDiff)
		} else {
			bp.TotalSupply = new(big.Int).Add(bp.TotalSupply, totalSupplyDiff)
		}

	}

	// emit events
	encoded, err := call.Encode()
	handleError(err)

	a.fch.emitEvent(
		types.NewTransactionExecutedEvent(
			bp.Height,
			encoded,
			callHash,
			res,
		),
	)
	a.fch.emitEvent(types.NewBlockExecutedEvent(bp))

	// commit block proposal
	err = a.fch.blockstore.CommitBlockProposal()
	handleError(err)

	return res
}

func (a *Account) checkAuthorized() {
	// check if account is authorized (i.e. is a bridged account)
	if !a.isAuthorized {
		handleError(types.ErrUnAuthroizedMethodCall)
	}
}

func handleError(err error) {
	if err == nil {
		return
	}

	if types.IsAFatalError(err) {
		// don't wrap it
		panic(err)
	}
	panic(errors.NewEVMError(err))
}
