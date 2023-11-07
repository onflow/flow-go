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
	blockchain    types.BlockChain
	backend       types.Backend
	emulator      types.Emulator
	newBlockDraft *types.Block
}

var _ types.ContractHandler = &ContractHandler{}

func NewContractHandler(
	blockchain types.BlockChain,
	backend types.Backend,
	emulator types.Emulator,
) *ContractHandler {
	return &ContractHandler{
		blockchain: blockchain,
		backend:    backend,
		emulator:   emulator,
	}
}

// this method has been separated out to make NewContractHandler lighter
// and lazy load these operations if needed
func (h *ContractHandler) setupNewBlockDraft() {
	lastExecutedBlock, err := h.blockchain.LatestBlock()
	handleError(err)

	parentHash, err := lastExecutedBlock.Hash()
	if err != nil {
		// this is a fatal error
		panic(err)
	}
	h.newBlockDraft = &types.Block{
		Height:            lastExecutedBlock.Height + 1,
		ParentBlockHash:   parentHash,
		UUIDIndex:         lastExecutedBlock.UUIDIndex,
		TotalSupply:       lastExecutedBlock.TotalSupply,
		TransactionHashes: make([]gethCommon.Hash, 0),
	}
}

func (h *ContractHandler) allocateUUID() uint64 {
	if h.newBlockDraft == nil {
		h.setupNewBlockDraft()
	}
	uuid := h.newBlockDraft.UUIDIndex
	h.newBlockDraft.UUIDIndex = uuid + 1
	return uuid
}

func (h *ContractHandler) updateBlockDraftStateRoot(stateRoot gethCommon.Hash) {
	if h.newBlockDraft == nil {
		h.setupNewBlockDraft()
	}
	h.newBlockDraft.StateRoot = stateRoot
}

func (h *ContractHandler) updateBlockDraftTotalSupply(newValue uint64) {
	if h.newBlockDraft == nil {
		h.setupNewBlockDraft()
	}
	h.newBlockDraft.TotalSupply = newValue
}

func (h *ContractHandler) getBlockDraftTotalSupply() uint64 {
	if h.newBlockDraft == nil {
		h.setupNewBlockDraft()
	}
	return h.newBlockDraft.Height
}

func (h *ContractHandler) appendTxHashToBlockDraft(txHash gethCommon.Hash) {
	if h.newBlockDraft == nil {
		h.setupNewBlockDraft()
	}
	h.newBlockDraft.TransactionHashes = append(h.newBlockDraft.TransactionHashes, txHash)
}

func (h *ContractHandler) getBlockDraftHeight() uint64 {
	if h.newBlockDraft == nil {
		h.setupNewBlockDraft()
	}
	return h.newBlockDraft.Height
}

func (h *ContractHandler) commitBlockDraft() {
	err := h.blockchain.AppendBlock(h.newBlockDraft)
	handleError(err)
	h.EmitEvent(types.NewBlockExecutedEvent(h.newBlockDraft))
	// reset it
	h.newBlockDraft = nil
}

// AllocateAddress allocates an address to be used by the bridged accounts
//
// TODO: future improvement: check for collision if account exist try a new account
// TODO: if we allocate address but don't do anything else the state root would stay the same, does it cause issue?
// maybe uuid index should be outside of the scope of a block
func (h *ContractHandler) AllocateAddress() types.Address {
	target := types.Address{}
	// first 12 bytes would be zero
	// the next 8 bytes would be an increment of the UUID index
	binary.BigEndian.PutUint64(target[12:], h.allocateUUID())
	return target
}

// AccountByAddress returns the account for the given address,
// if isAuthorized is set, account is controlled by the FVM (bridged accounts)
func (h *ContractHandler) AccountByAddress(addr types.Address, isAuthorized bool) types.Account {
	return newAccount(h, addr, isAuthorized)
}

// LastExecutedBlock returns the last executed block
func (h ContractHandler) LastExecutedBlock() *types.Block {
	block, err := h.blockchain.LatestBlock()
	handleError(err)
	return block
}

// Run runs an rlpencoded evm transaction, collect the evm fees under the provided coinbase
func (h ContractHandler) Run(rlpEncodedTx []byte, coinbase types.Address) bool {
	// Decode transaction encoding
	tx := gethTypes.Transaction{}

	err := h.backend.MeterComputation(environment.ComputationKindRLPDecoding, uint(len(rlpEncodedTx)))
	handleError(err)

	err = tx.DecodeRLP(
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
	txHash := tx.Hash()
	h.appendTxHashToBlockDraft(txHash)
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
		h.getBlockDraftHeight(),
		rlpEncodedTx,
		txHash,
		res,
	))
	h.updateBlockDraftStateRoot(res.StateRootHash)
	h.commitBlockDraft()
	return !failed
}

func (h *ContractHandler) captureCall(call *types.DirectCall) ([]byte, gethCommon.Hash) {
	hash, err := call.Hash()
	if err != nil {
		// this is fatal
		panic(err)
	}
	h.appendTxHashToBlockDraft(hash)
	encoded, err := call.Encode()
	handleError(err)
	return encoded, hash
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

func (h *ContractHandler) getBlockContext() types.BlockContext {
	return types.BlockContext{
		BlockNumber:            h.getBlockDraftHeight(),
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
// and update the account balance with the new amount
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
			a.fch.getBlockDraftHeight(),
			encoded,
			callHash,
			res,
		),
	)
	handleError(err)
	newBalance := a.fch.getBlockDraftTotalSupply() + v.Balance().ToAttoFlow().Uint64()
	a.fch.updateBlockDraftTotalSupply(newBalance)
	a.fch.updateBlockDraftStateRoot(res.StateRootHash)
	a.fch.commitBlockDraft()

}

// Withdraw deducts the balance from the account and
// withdraw and return flow token from the Flex main vault.
func (a *Account) Withdraw(b types.Balance) *types.FLOWTokenVault {
	a.checkAuthorized()

	totalSupply := a.fch.getBlockDraftTotalSupply()
	// check balance of flex vault
	if b.ToAttoFlow().Uint64() > totalSupply {
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
			a.fch.getBlockDraftHeight(),
			encoded,
			callHash,
			res,
		),
	)
	handleError(err)

	// emit event
	a.fch.updateBlockDraftTotalSupply(totalSupply - b.ToAttoFlow().Uint64())
	a.fch.updateBlockDraftStateRoot(res.StateRootHash)
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
	res := a.executeAndHandleCall(a.fch.getBlockContext(), call)
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
			a.fch.getBlockDraftHeight(),
			encoded,
			callHash,
			res,
		),
	)
	a.fch.updateBlockDraftStateRoot(res.StateRootHash)
	a.fch.commitBlockDraft()
	return res
}

func (a *Account) checkAuthorized() {
	// check if account is authorized (i.e. is a bridged account)
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
