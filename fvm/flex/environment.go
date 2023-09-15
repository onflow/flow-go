package flex

import (
	"math/big"

	"github.com/onflow/flow-go/fvm/flex/storage"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
)

// Environment is a one-time use flex environment and
// should not be used more than once
type Environment struct {
	Config   *Config
	EVM      *vm.EVM
	Database *storage.Database
	State    *state.StateDB
	Result   *Result
	Used     bool
}

// NewEnvironment constructs a new Flex Enviornment
func NewEnvironment(
	cfg *Config,
	db *storage.Database,
) (*Environment, error) {

	rootHash, err := db.GetRootHash()
	if err != nil {
		return nil, err
	}

	execState, err := state.New(rootHash,
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
		Database: db,
		State:    execState,
		Result:   &Result{},
		Used:     false,
	}, nil
}

func (fe *Environment) checkExecuteOnce() error {
	if fe.Used {
		return ErrFlexEnvReuse
	}
	fe.Used = true
	return nil
}

func (fe *Environment) updateGasConsumed(leftOverGas uint64) {
	fe.Result.GasConsumed = fe.Config.BlockContext.GasLimit - leftOverGas
}

// commit commits the changes to the state.
// if error is returned is a fatal one.
func (fe *Environment) commit() error {
	// commit the changes
	var height uint64
	if fe.Config.BlockContext.BlockNumber != nil {
		height = fe.Config.BlockContext.BlockNumber.Uint64()
	}

	// commits the changes from the journal into the in memory trie.
	newRoot, err := fe.State.Commit(height, true)
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

	err = fe.Database.SetRootHash(newRoot)
	if err != nil {
		return err
	}

	// TODO: emit event on root changes
	fe.Result.RootHash = newRoot
	return nil
}

// MintTo mints tokens into the target address, if the address dees not
// exist it would create it first.
//
// Warning, This method should only be used for bridging native token into Flex
// from to the FVM environment.
func (fe *Environment) MintTo(balance *big.Int, target common.Address) error {

	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	if fe.Config.BlockContext.GasLimit < TransferGasUsage {
		// still we deduct the amount that was possible to use
		fe.Result.GasConsumed = fe.Config.BlockContext.GasLimit
		return ErrGasLimit
	}

	// add address to the access list ?
	fe.State.AddAddressToAccessList(target)
	// create account (TODO: check if exists)
	fe.State.CreateAccount(target)
	// add balance
	fe.State.AddBalance(target, balance)

	// we don't need to increment any nonce, given the origin doesn't exist

	// update the gas consumed
	fe.Result.GasConsumed = TransferGasUsage

	// TODO: emit an event

	return fe.commit()
}

// WithdrawFrom deduct the balance from the given source account.
//
// Warning, This method should only be used for bridging native token from Flex
// back to the FVM environment. This method should only be used for FOA's, for other
// accounts they should send a message to transfer money to account zero?
func (fe *Environment) WithdrawFrom(balance *big.Int, source common.Address) error {

	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	if fe.Config.BlockContext.GasLimit < TransferGasUsage {
		// still we deduct the amount that was possible to use
		fe.Result.GasConsumed = fe.Config.BlockContext.GasLimit
		return ErrGasLimit
	}

	// // TODO check account exist
	// if !fe.State.Exist(source) {
	// 	// return not exist
	// }

	// add address to the access list ?
	fe.State.AddAddressToAccessList(source)
	// add balance
	fe.State.SubBalance(source, balance)

	// we increment the nonce for source account cause
	// withdraw counts as a
	nonce := fe.State.GetNonce(source)
	fe.State.SetNonce(source, nonce+1)

	// update the gas consumed
	fe.Result.GasConsumed = TransferGasUsage

	// TODO: emit an event

	return fe.commit()
}

// Deploy deploys a contract at the given address
// the value passed to this method would be deposited on the contract account
func (fe *Environment) Deploy(caller common.Address, code []byte, value *big.Int) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	// Execute the preparatory steps for state transition which includes:
	rules := fe.Config.Rules()
	fe.State.Prepare(rules,
		fe.Config.TxContext.Origin,
		fe.Config.BlockContext.Coinbase,
		nil,
		vm.ActivePrecompiles(rules),
		nil)

	// TODO maybe we should use Create2 and pass salt of type *uint256.Int
	ret, addr, leftOverGas, err := fe.EVM.Create(
		vm.AccountRef(caller),
		code,
		fe.Config.BlockContext.GasLimit,
		value)

	fe.Result.RetValue = ret
	fe.Result.DeployedContractAddress = addr
	fe.updateGasConsumed(leftOverGas)
	return err
}

// TODO update read me
func (fe *Environment) Call(caller common.Address, contract common.Address, input []byte, value *big.Int) error {
	if err := fe.checkExecuteOnce(); err != nil {
		return err
	}

	rules := fe.Config.Rules()
	fe.State.Prepare(rules,
		caller,
		fe.Config.BlockContext.Coinbase,
		&contract,
		vm.ActivePrecompiles(rules),
		nil)

	// Call the code with the given configuration.
	ret, leftOverGas, err := fe.EVM.Call(
		fe.State.GetOrNewStateObject(caller),
		contract,
		input,
		fe.Config.BlockContext.GasLimit,
		value,
	)
	if err != nil {
		return err
	}

	fe.Result.RetValue = ret
	fe.updateGasConsumed(leftOverGas)
	// TODO deal with the logs
	// TODO process error (fatal vs non fatal)

	// we increment the nonce for origin account cause
	// withdraw counts as a
	// TODO: check if the call is making the adjustment to the Nonce or not
	// TODO: check if we need to move this logic before the call.
	nonce := fe.State.GetNonce(caller)
	fe.State.SetNonce(caller, nonce+1)

	return fe.commit()
}

// TODO
func (fe *Environment) RunFlexTransaction() error {
	return nil
}
