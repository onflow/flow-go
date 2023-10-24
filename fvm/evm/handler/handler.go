package handler

import (
	"bytes"
	"encoding/binary"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/types"
)

// ContractHandler is responsible for triggering calls to emulator, metering,
// event emission and updating the block
//
// TODO and Warning: currently database keeps a copy of roothash, and if after
// commiting the changes by the evm we want to revert in this code we need to reset that
// or we should always do all the checks and return before calling the emulator,
// after that should be only event emissions and computation usage updates.
// thats another reason we first check the computation limit before using.
// in the future we might benefit from a view style of access to db passed as
// a param to the emulator.
type ContractHandler struct {
	blockchain        types.BlockChain
	backend           types.Backend
	emulator          types.Emulator
	lastExecutedBlock *types.Block
	uuidIndex         uint64
	totalSupply       uint64
	transactions      []gethCommon.Hash
}

var _ types.ContractHandler = &ContractHandler{}

func NewContractHandler(
	blockchain types.BlockChain,
	backend types.Backend,
	emulator types.Emulator,
) *ContractHandler {
	lastExecutedBlock, err := blockchain.LatestBlock()
	// fatal error
	if err != nil {
		panic(err)
	}

	return &ContractHandler{
		blockchain:        blockchain,
		backend:           backend,
		emulator:          emulator,
		lastExecutedBlock: lastExecutedBlock,
		uuidIndex:         lastExecutedBlock.UUIDIndex,
		totalSupply:       lastExecutedBlock.TotalSupply,
		transactions:      make([]gethCommon.Hash, 0),
	}
}

// AllocateAddress allocates an address to be used by FOA resources
func (h *ContractHandler) AllocateAddress() types.Address {
	target := types.Address{}
	// first 12 bytes would be zero
	// the next 8 bytes would be incremented of uuid
	binary.BigEndian.PutUint64(target[12:], h.lastExecutedBlock.UUIDIndex)
	h.uuidIndex++

	// TODO: if account exist try some new number
	// if fe.State.Exist(target.ToCommon()) {
	// }

	h.updateLastExecutedBlock(h.lastExecutedBlock.StateRoot, gethTypes.EmptyRootHash)
	return target
}

// AccountByAddress returns the account for the given address,
// if isAuthorized is set, account is controlled by the FVM and FOA resources
func (h *ContractHandler) AccountByAddress(addr types.Address, isAuthorized bool) types.Account {
	return newAccount(h, addr, isAuthorized)
}

// LastExecutedBlock returns the last executed block
func (h ContractHandler) LastExecutedBlock() *types.Block {
	block, err := h.blockchain.LatestBlock()
	handleError(err)
	return block
}

func (h *ContractHandler) updateLastExecutedBlock(stateRoot, eventRoot gethCommon.Hash) {
	h.lastExecutedBlock = types.NewBlock(
		h.lastExecutedBlock.Height+1,
		h.uuidIndex,
		h.totalSupply,
		stateRoot,
		eventRoot,
		h.transactions,
	)

	err := h.blockchain.AppendBlock(h.lastExecutedBlock)
	handleError(err)
}

// Run runs an rlpencoded evm transaction, collect the evm fees under the provided coinbase
func (h ContractHandler) Run(rlpEncodedTx []byte, coinbase types.Address) bool {
	// Decode transaction encoding
	tx := gethTypes.Transaction{}
	// TODO: update the max limit on the encoded size to a meaningful value
	err := tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(rlpEncodedTx),
			uint64(len(rlpEncodedTx))))
	handleError(err)

	// check tx gas limit
	gasLimit := tx.Gas()
	h.checkGasLimit(types.GasLimit(gasLimit))

	ctx := h.getBlockContext()
	ctx.GasFeeCollector = coinbase

	blk, err := h.emulator.NewBlockView(ctx)
	handleError(err)

	res, err := blk.RunTransaction(&tx)
	txHash := h.captureTx(&tx)
	h.meterGasUsage(res)

	failed := false
	if err != nil {
		// if error is fatal panic here
		if types.IsAFatalError(err) {
			// don't wrap it
			panic(err)
		}
		err = errors.NewEVMError(err)
		failed = true
	}
	if res == nil {
		// fatal error
		panic("empty result is retuned by emulator")
	}

	res.Failed = failed

	h.EmitEvent(types.NewTransactionExecutedEvent(
		h.lastExecutedBlock.Height+1,
		rlpEncodedTx,
		txHash,
		res,
	))
	h.EmitLastExecutedBlockEvent()
	h.updateLastExecutedBlock(res.StateRootHash, gethTypes.EmptyRootHash)
	return !failed
}

func (h ContractHandler) checkGasLimit(limit types.GasLimit) {
	// check gas limit against what has been left on the transaction side
	if !h.backend.ComputationAvailable(environment.ComputationKindEVMGasUsage, uint(limit)) {
		handleError(types.ErrInsufficientComputation)
	}
}

func (h ContractHandler) meterGasUsage(res *types.Result) {
	if res != nil {
		err := h.backend.MeterComputation(environment.ComputationKindEVMGasUsage, uint(res.GasConsumed))
		handleError(err)
	}
}

func (h *ContractHandler) EmitEvent(event *types.Event) {
	// TODO add extra metering for rlp encoding
	encoded, err := event.Payload.Encode()
	handleError(err)
	h.backend.EmitFlowEvent(event.Etype, encoded)
}

func (h *ContractHandler) captureTx(tx *gethTypes.Transaction) gethCommon.Hash {
	hash := tx.Hash()
	h.transactions = append(h.transactions, hash)
	return hash
}

func (h *ContractHandler) captureCall(call *types.DirectCall) ([]byte, gethCommon.Hash) {
	hash, err := call.Hash()
	if err != nil {
		// this is fatal
		panic(err)
	}
	h.transactions = append(h.transactions, hash)
	encoded, err := call.Encode()
	handleError(err)
	return encoded, hash
}

func (h *ContractHandler) EmitLastExecutedBlockEvent() {
	block, err := h.blockchain.LatestBlock()
	handleError(err)
	h.EmitEvent(types.NewBlockExecutedEvent(block))
}

func (h *ContractHandler) getBlockContext() types.BlockContext {
	return types.BlockContext{
		BlockNumber:            h.lastExecutedBlock.Height + 1,
		DirectCallBaseGasUsage: types.DefaultDirectCallBaseGasUsage,
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
// and update the FOA balance with the new amount
func (a *Account) Deposit(v *types.FLOWTokenVault) {
	cfg := a.fch.getBlockContext()
	a.fch.checkGasLimit(types.GasLimit(cfg.DirectCallBaseGasUsage))

	blk, err := a.fch.emulator.NewBlockView(cfg)
	handleError(err)

	call := types.NewDepositCall(
		a.address,
		v.Balance().ToAttoFlow(),
	)
	res, err := blk.DirectCall(call)
	a.fch.meterGasUsage(res)
	encoded, callHash := a.fch.captureCall(call)
	a.fch.EmitEvent(
		types.NewTransactionExecutedEvent(
			a.fch.lastExecutedBlock.Height+1,
			encoded,
			callHash,
			res,
		),
	)
	handleError(err)
	// emit event
	a.fch.EmitLastExecutedBlockEvent()
	a.fch.totalSupply += v.Balance().ToAttoFlow().Uint64()
	a.fch.updateLastExecutedBlock(res.StateRootHash, gethTypes.EmptyRootHash)
}

// Withdraw deducts the balance from the FOA account and
// withdraw and return flow token from the Flex main vault.
func (a *Account) Withdraw(b types.Balance) *types.FLOWTokenVault {
	a.checkAuthorized()

	// check balance of flex vault
	if b.ToAttoFlow().Uint64() > a.fch.totalSupply {
		handleError(types.ErrInsufficientTotalSupply)
	}

	cfg := a.fch.getBlockContext()
	a.fch.checkGasLimit(types.GasLimit(cfg.DirectCallBaseGasUsage))
	blk, err := a.fch.emulator.NewBlockView(cfg)
	handleError(err)

	call := types.NewWithdrawCall(
		a.address,
		b.ToAttoFlow(),
	)
	res, err := blk.DirectCall(call)
	a.fch.meterGasUsage(res)
	encoded, callHash := a.fch.captureCall(call)
	a.fch.EmitEvent(
		types.NewTransactionExecutedEvent(
			a.fch.lastExecutedBlock.Height+1,
			encoded,
			callHash,
			res,
		),
	)
	handleError(err)

	// emit event
	a.fch.EmitLastExecutedBlockEvent()
	a.fch.totalSupply -= b.ToAttoFlow().Uint64()
	a.fch.updateLastExecutedBlock(res.StateRootHash, gethTypes.EmptyRootHash)
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
	a.executeAndHandleCall(ctx, call)
}

// Deploy deploys a contract to the EVM environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the FOA accounts
func (a *Account) Deploy(code types.Code, gaslimit types.GasLimit, balance types.Balance) types.Address {
	a.checkAuthorized()
	a.fch.checkGasLimit(gaslimit)
	call := types.NewDeployCall(
		a.address,
		code,
		uint64(gaslimit),
		balance.ToAttoFlow(),
	)
	res := a.executeAndHandleCall(a.fch.getBlockContext(), call)
	return types.Address(res.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
// contract data is not controlled by the FOA accounts
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
	res := a.executeAndHandleCall(a.fch.getBlockContext(), call)
	return res.ReturnedValue
}

func (a *Account) executeAndHandleCall(
	ctx types.BlockContext,
	call *types.DirectCall,
) *types.Result {
	blk, err := a.fch.emulator.NewBlockView(ctx)
	handleError(err)

	res, err := blk.DirectCall(call)
	a.fch.meterGasUsage(res)
	encoded, callHash := a.fch.captureCall(call)
	handleError(err)

	a.fch.EmitEvent(
		types.NewTransactionExecutedEvent(
			a.fch.lastExecutedBlock.Height+1,
			encoded,
			callHash,
			res,
		),
	)
	a.fch.EmitLastExecutedBlockEvent()
	// TODO: update this to calculate receipt hash
	a.fch.updateLastExecutedBlock(res.StateRootHash, gethTypes.EmptyRootHash)
	return res
}

func (a *Account) checkAuthorized() {
	// check if account is authorized to to FOA related opeartions
	if !a.isAuthorized {
		handleError(types.ErrUnAuthroizedMethodCall)
	}
}

func handleError(err error) {
	if err != nil {
		if types.IsAFatalError(err) {
			// don't wrap it
			panic(err)
		}
		panic(errors.NewEVMError(err))
	}
}
