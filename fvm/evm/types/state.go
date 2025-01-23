package types

import (
	"github.com/holiman/uint256"
	"github.com/onflow/crypto/hash"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/onflow/go-ethereum/rlp"
)

// StateDB acts as the main interface to the EVM runtime
type StateDB interface {
	gethVM.StateDB

	// Commit commits the changes and
	// returns a commitment over changes
	// setting `finalize` flag
	// calls a subsequent call to Finalize
	// deferring finalization and calling it once at the end
	// improves efficiency of batch operations.
	Commit(finalize bool) (hash.Hash, error)

	// Finalize flushes all the changes
	// to the permanent storage
	Finalize() error

	// Logs collects and prepares logs
	Logs(
		blockNumber uint64,
		txHash gethCommon.Hash,
		txIndex uint,
	) []*gethTypes.Log

	// Preimages returns a map of preimages
	Preimages() map[gethCommon.Hash][]byte

	// Reset resets uncommitted changes and transient artifacts such as error, logs,
	// preimages, access lists, ...
	// The method is often called between execution of different transactions
	Reset()

	// Error returns any error that has been cached so far by the state.
	Error() error
}

// ReadOnlyView provides a readonly view of the state
type ReadOnlyView interface {
	// Exist returns true if the address exist in the state
	Exist(gethCommon.Address) (bool, error)
	// IsCreated returns true if address has been created in this tx
	IsCreated(gethCommon.Address) bool
	// IsNewContract returns true if address is a new contract
	// either is a new account or it had balance but no code before
	IsNewContract(addr gethCommon.Address) bool
	// HasSelfDestructed returns true if an address has self destructed
	// it also returns the balance of address before selfdestruction call
	HasSelfDestructed(gethCommon.Address) (bool, *uint256.Int)
	// GetBalance returns the balance of an address
	GetBalance(gethCommon.Address) (*uint256.Int, error)
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
	// GetStorageRoot returns some sort of root for the given address.
	// Warning! Since StateDB doesn't construct a Merkel tree under the hood,
	// the behavior of this endpoint is as follow:
	// - if an account doesn't exist it returns common.Hash{}
	// - if account is EOA it returns gethCommon.EmptyRootHash
	// - else it returns a unique hash value as the root but this returned
	GetStorageRoot(gethCommon.Address) (gethCommon.Hash, error)
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
	// CreateContract is used whenever a contract is created. This may be preceded
	// by CreateAccount, but that is not required if it already existed in the
	// state due to funds sent beforehand.
	CreateContract(gethCommon.Address)
	// SelfDestruct set the flag for destruction of the account after execution
	SelfDestruct(gethCommon.Address) error

	// SubBalance subtracts the amount from the balance the given address
	SubBalance(gethCommon.Address, *uint256.Int) error
	// AddBalance adds the amount to the balance of the given address
	AddBalance(gethCommon.Address, *uint256.Int) error
	// SetNonce sets the nonce for the given address
	SetNonce(gethCommon.Address, uint64) error
	// SetCode sets the code for the given address
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
}

// BaseView is a low-level mutable view of the state
// baseview is usually updated at the commit calls to the higher level view
type BaseView interface {
	ReadOnlyView

	// Creates a new account
	CreateAccount(
		addr gethCommon.Address,
		balance *uint256.Int,
		nonce uint64,
		code []byte,
		codeHash gethCommon.Hash,
	) error

	// UpdateAccount updates a account
	UpdateAccount(
		addr gethCommon.Address,
		balance *uint256.Int,
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

// SlotEntry captures an address to a storage slot and the value stored in it
type SlotEntry struct {
	Address gethCommon.Address
	Key     gethCommon.Hash
	Value   gethCommon.Hash
}

// Encoded returns the encoded content of the slot entry
func (se *SlotEntry) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(se)
}

// SlotEntryFromEncoded constructs an slot entry from the encoded data
func SlotEntryFromEncoded(encoded []byte) (*SlotEntry, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	se := &SlotEntry{}
	return se, rlp.DecodeBytes(encoded, se)
}
