package env

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	"github.com/ethereum/go-ethereum/rlp"
)

// Environment is a one-time use flex environment and
// should not be used more than once
type Environment struct {
	Config            *Config
	EVM               *vm.EVM
	Database          *storage.Database
	State             *state.StateDB
	LastExecutedBlock *models.FlexBlock
	Result            *Result
	Used              bool
}

// NewEnvironment constructs a new Flex Enviornment
func NewEnvironment(
	cfg *Config,
	db *storage.Database,
) (*Environment, error) {

	lastExcutedBlock, err := db.GetLatestBlock()
	if err != nil {
		return nil, err
	}

	execState, err := state.New(lastExcutedBlock.StateRoot,
		state.NewDatabase(
			rawdb.NewDatabase(db),
		),
		nil)
	if err != nil {
		return nil, err
	}

	return &Environment{
		Config: cfg,
		EVM: vm.NewEVM(
			*cfg.BlockContext,
			*cfg.TxContext,
			execState,
			cfg.ChainConfig,
			cfg.EVMConfig,
		),
		Database:          db,
		State:             execState,
		LastExecutedBlock: lastExcutedBlock,
		Result: &Result{
			UUIDIndex:                lastExcutedBlock.UUIDIndex,
			TotalSupplyOfNativeToken: lastExcutedBlock.TotalSupply,
		},
		Used: false,
	}, nil
}

func (fe *Environment) checkExecuteOnce() error {
	if fe.Used {
		return ErrFlexEnvReuse
	}
	fe.Used = true
	return nil
}

// commit commits the changes to the state.
// if error is returned is a fatal one.
func (fe *Environment) commit() error {
	// commit the changes
	// ramtin: height is needed when we want to update to version v13
	// var height uint64
	// if fe.Config.BlockContext.BlockNumber != nil {
	// 	height = fe.Config.BlockContext.BlockNumber.Uint64()
	// }

	// commits the changes from the journal into the in memory trie.
	newRoot, err := fe.State.Commit(true)
	if err != nil {
		return err
	}

	// flush the trie to the lower level db
	// the reason we have to do this, is the original database
	// is designed to keep changes in memory until the state.Commit
	// is called then the changes moves into the trie, but the trie
	// would stay in memory for faster trasnaction execution. you
	// have to explicitly ask the trie to commit to the underlying storage
	err = fe.State.Database().TrieDB().Commit(newRoot, false)
	if err != nil {
		return err
	}

	newBlock := models.NewFlexBlock(
		fe.LastExecutedBlock.Height+1,
		fe.Result.UUIDIndex,
		fe.Result.TotalSupplyOfNativeToken,
		newRoot,
		types.EmptyRootHash,
	)

	err = fe.Database.SetLatestBlock(newBlock)
	if err != nil {
		return err
	}

	// TODO: emit event on root changes
	fe.Result.RootHash = newRoot
	return nil
}

// TODO: properly use an address generator (zeros + random section) and verify collision
// TODO: does this leads to trie depth issue?
func (fe *Environment) AllocateAddressAndMintTo(balance *big.Int) (*models.FlexAddress, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}

	target := fe.allocateAddress()
	fe.mintTo(balance, target.ToCommon())

	// TODO: emit an event

	return target, fe.commit()
}

// Balance returns the balance of an address
// TODO: do it as a ReadOnly env view.
func (fe *Environment) Balance(target *models.FlexAddress) (*big.Int, error) {
	if err := fe.checkExecuteOnce(); err != nil {
		return nil, err
	}
	return fe.State.GetBalance(target.ToCommon()), nil
}

func (fe *Environment) allocateAddress() *models.FlexAddress {
	target := models.FlexAddress{}
	// first 12 bytes would be zero
	// the next 8 bytes would be incremented of uuid
	binary.BigEndian.PutUint64(target[12:], fe.LastExecutedBlock.UUIDIndex)
	fe.Result.UUIDIndex++

	// TODO: if account exist try some new number
	// if fe.State.Exist(target.ToCommon()) {
	// }

	return &target
}

// MintTo mints tokens into the target address, if the address dees not
// exist it would create it first.
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) MintTo(balance *big.Int, target common.Address) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	fe.mintTo(balance, target)

	return fe.commit()
}

func (fe *Environment) mintTo(balance *big.Int, target common.Address) {
	// update the gas consumed // TODO: revisit
	// do it as the very first thing to prevent attacks
	fe.Result.GasConsumed = TransferGasUsage

	// check account if not exist
	if !fe.State.Exist(target) {
		fe.State.CreateAccount(target)
	}

	// add balance
	fe.State.AddBalance(target, balance)
	fe.Result.TotalSupplyOfNativeToken += balance.Uint64()

	// we don't need to increment any nonce, given the origin doesn't exist

	// TODO: emit an event
}

// WithdrawFrom deduct the balance from the given source account.
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) WithdrawFrom(amount *big.Int, source common.Address) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	if amount.Uint64() > fe.Result.TotalSupplyOfNativeToken {
		return fmt.Errorf("total supply does not match %d > %d", amount.Uint64(), fe.Result.TotalSupplyOfNativeToken)
	}

	// update the gas consumed // TODO: revisit
	// do it as the very first thing to prevent attacks
	fe.Result.GasConsumed = TransferGasUsage

	// check account exist
	if !fe.State.Exist(source) {
		fe.Result.Failed = true
		return nil
	}

	// check balance
	// if balance is lower than amount return
	if fe.State.GetBalance(source).Cmp(amount) == -1 {
		fe.Result.Failed = true
		return nil
	}

	// add balance
	fe.State.SubBalance(source, amount)
	fe.Result.TotalSupplyOfNativeToken -= amount.Uint64()

	// we increment the nonce for source account cause
	// withdraw counts as a transaction (similar to the way calls increment the nonce)
	nonce := fe.State.GetNonce(source)
	fe.State.SetNonce(source, nonce+1)

	// TODO: emit an event

	return fe.commit()
}

// Transfer transfers flow token from an FOA account to another flex account
// this is a similar functionality as calling a call with empty data,
// mostly provided for a easier interaction
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) Transfer(
	from *common.Address,
	to *common.Address,
	value *big.Int,
) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}
	msg := directCallMessage(from, to, value, nil, DefaultMaxGasLimit)
	return fe.run(msg)
}

// Deploy deploys a contract at the given address
// the value passed to this method would be deposited on the contract account
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) Deploy(
	caller common.Address,
	code []byte,
	gasLimit uint64,
	value *big.Int,
) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}
	msg := directCallMessage(&caller, nil, value, code, gasLimit)
	return fe.run(msg)
}

// Call calls a smart contract with the input
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's
// accounts where resource ownership has been verified
func (fe *Environment) Call(
	from common.Address,
	to common.Address,
	data []byte,
	gasLimit uint64,
	value *big.Int,
) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}
	// TODO: verify that the authorizer has the resource to interact with this contract (higher level check)

	msg := directCallMessage(&from, &to, value, data, gasLimit)

	return fe.run(msg)
}

// RunTransaction runs a flex transaction
// this method could be called by anyone.
// TODO : check gas limit complience (one set on tx and the one allowed by flow tx)
func (fe *Environment) RunTransaction(rlpEncodedTx []byte) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	// decode rlp
	tx := types.Transaction{}
	// TODO: update the max limit on the encoded size to a meaningful value
	err := tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(rlpEncodedTx),
			uint64(len(rlpEncodedTx))))
	if err != nil {
		return err
	}

	signer := types.MakeSigner(fe.Config.ChainConfig, BlockNumberForEVMRules, fe.Config.BlockContext.Time)

	msg, err := core.TransactionToMessage(&tx, signer, fe.Config.BlockContext.BaseFee)
	if err != nil {
		return err
	}

	return fe.run(msg)
}

func (fe *Environment) run(msg *core.Message) error {
	execResult, err := core.NewStateTransition(fe.EVM, msg, (*core.GasPool)(&fe.Config.BlockContext.GasLimit)).TransitionDb()
	if err != nil {
		return err
	}

	fe.Result.RetValue = execResult.ReturnData
	fe.Result.GasConsumed = execResult.UsedGas
	fe.Result.Failed = execResult.Failed()
	fe.Result.Logs = fe.State.Logs()

	// If the transaction created a contract, store the creation address in the receipt.
	// TODO verify this later
	if msg.To == nil {
		fe.Result.DeployedContractAddress = crypto.CreateAddress(msg.From, msg.Nonce)
	}

	// TODO check if we have the logic to pay the coinbase
	return fe.commit()
}

func directCallMessage(
	from, to *common.Address,
	value *big.Int,
	data []byte,
	gasLimit uint64,
) *core.Message {
	return &core.Message{
		To:        to,
		From:      *from,
		Value:     value,
		Data:      data,
		GasLimit:  gasLimit,
		GasPrice:  big.NewInt(0), // price has to be zero
		GasTipCap: big.NewInt(1), // TODO revisit this value (also known as maxPriorityFeePerGas)
		GasFeeCap: big.NewInt(2), // TODO revisit this value (also known as maxFeePerGas)
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}
