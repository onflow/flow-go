package types

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
)

// StateDB is acts as the main interface to the EVM runtime
type StateDB interface {
	gethVM.StateDB

	// Commit commits the changes
	Commit() error

	// Logs collects and prepares logs
	Logs(
		blockHash gethCommon.Hash,
		blockNumber uint64,
		txHash gethCommon.Hash,
		txIndex uint,
	) []*gethTypes.Log

	// returns a map of preimages
	Preimages() map[gethCommon.Hash][]byte

	// Reset prepares the storage for the next flow transaction execution
	Reset() error
}

// ReadOnlyView provides a readonly view of the state
type ReadOnlyView interface {
	// Exist returns true if an address exist in the state
	Exist(gethCommon.Address) (bool, error)
	// IsCreated returns true if address has been created in this tx
	IsCreated(gethCommon.Address) bool
	// HasSuicided returns true if an address has suicided
	HasSuicided(gethCommon.Address) bool
	// GetBalance returns the balance of an address
	GetBalance(gethCommon.Address) (*big.Int, error)
	// GetNonce returns the nonce of an address
	GetNonce(gethCommon.Address) (uint64, error)
	// GetCode returns the code of an address
	GetCode(gethCommon.Address) ([]byte, error)
	// GetCodeHash returns the code hash of an address
	GetCodeHash(gethCommon.Address) (gethCommon.Hash, error)
	// GetCodeSize returns the code size of an address
	GetCodeSize(gethCommon.Address) (int, error)
	// GetState returns values for an slot in the main storage
	GetState(SlotAddress) (gethCommon.Hash, error)
	// GetTransientState returns values for an slot transient storage
	GetTransientState(SlotAddress) gethCommon.Hash
	// GetRefund returns the total amount of (gas) refund
	GetRefund() uint64
	// AddressInAccessList checks if an address is in the access list
	AddressInAccessList(gethCommon.Address) bool
	// SlotInAccessList checks if a slot is in the access list
	SlotInAccessList(SlotAddress) (addressOk bool, slotOk bool)
}

// HotView captures a high-level mutable view of the state
type HotView interface {
	ReadOnlyView

	// CreateAccount creates a new account
	CreateAccount(gethCommon.Address) error
	// Suicide set the flag for deletion of the account after execution
	Suicide(gethCommon.Address) (success bool, err error)
	// SubBalance subtracts the amount from the balance the given address
	SubBalance(gethCommon.Address, *big.Int) error
	// AddBalance adds the amount to the balance of the given address
	AddBalance(gethCommon.Address, *big.Int) error
	// SetNonce sets the nonce for the given address
	SetNonce(gethCommon.Address, uint64) error
	// SetNonce sets the code for the given address
	SetCode(gethCommon.Address, []byte) error

	// SetState sets a value for the given slot in the main storage
	SetState(SlotAddress, gethCommon.Hash) error
	// SetTransientState sets a value for the given slot in the transient storage
	SetTransientState(SlotAddress, gethCommon.Hash)

	// AddRefund adds the amount to the total (gas) refund
	AddRefund(uint64) error
	// SubRefund subtracts the amount from the total (gas) refund
	SubRefund(uint64) error

	// AddAddressToAccessList adds an address to the per-transaction access list
	AddAddressToAccessList(addr gethCommon.Address) (addressAdded bool)
	// AddSlotToAccessList adds a slot to the per-transaction access list
	AddSlotToAccessList(SlotAddress) (addressAdded, slotAdded bool)

	// AddLog append a log to the log collection
	AddLog(*gethTypes.Log)
	// AddPreimage adds a preimage to the list of preimages (input -> hash mapping)
	AddPreimage(gethCommon.Hash, []byte)

	// Commit finalizes and commits the changes
	Commit() error
}

// BaseView is a low-level mutable view of the state
// baseview is usually updated at the commit calls to the higher level view
type BaseView interface {
	ReadOnlyView

	// Creates a new account
	CreateAccount(
		addr gethCommon.Address,
		balance *big.Int,
		nonce uint64,
		code []byte,
		codeHash gethCommon.Hash,
	) error

	// UpdateAccount updates a account
	UpdateAccount(
		addr gethCommon.Address,
		balance *big.Int,
		nonce uint64,
		code []byte,
		codeHash gethCommon.Hash,
	) error

	// DeleteAccount deletes an account
	DeleteAccount(addr gethCommon.Address) error

	// UpdateSlot updates the value for the given slot in the main storage
	UpdateSlot(
		slot SlotAddress,
		value gethCommon.Hash,
	) error

	// Commit commits the changes
	Commit() error
}

// SlotAddress captures an address to a storage slot
type SlotAddress struct {
	Address gethCommon.Address
	Key     gethCommon.Hash
}
