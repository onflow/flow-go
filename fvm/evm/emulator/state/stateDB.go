package state

import (
	"bytes"
	stdErrors "errors"
	"fmt"
	"sort"

	"github.com/holiman/uint256"
	"github.com/onflow/atree"
	"github.com/onflow/crypto/hash"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethStateless "github.com/onflow/go-ethereum/core/stateless"
	gethTracing "github.com/onflow/go-ethereum/core/tracing"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethParams "github.com/onflow/go-ethereum/params"
	gethUtils "github.com/onflow/go-ethereum/trie/utils"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// StateDB implements a types.StateDB interface
//
// stateDB interface defined by the Geth doesn't support returning errors
// when state calls are happening, and requires stateDB to cache the error
// and return it at a later time (when commit is called). Only the first error
// is expected to be returned.
// Warning: current implementation of the StateDB is considered
// to be used for a single EVM transaction execution and is not
// thread safe. yet the current design supports addition of concurrency in the
// future if needed
type StateDB struct {
	ledger      atree.Ledger
	root        flow.Address
	baseView    types.BaseView
	views       []*DeltaView
	cachedError error
}

var _ types.StateDB = &StateDB{}

// NewStateDB constructs a new StateDB
func NewStateDB(ledger atree.Ledger, root flow.Address) (*StateDB, error) {
	bv, err := NewBaseView(ledger, root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		ledger:      ledger,
		root:        root,
		baseView:    bv,
		views:       []*DeltaView{NewDeltaView(bv)},
		cachedError: nil,
	}, nil
}

// Exist returns true if the given address exists in state.
//
// this should also return true for self destructed accounts during the transaction execution.
func (db *StateDB) Exist(addr gethCommon.Address) bool {
	exist, err := db.latestView().Exist(addr)
	db.handleError(err)
	return exist
}

// Empty returns whether the given account is empty.
//
// Empty is defined according to EIP161 (balance = nonce = code = 0).
func (db *StateDB) Empty(addr gethCommon.Address) bool {
	if !db.Exist(addr) {
		return true
	}
	return db.GetNonce(addr) == 0 &&
		db.GetBalance(addr).Sign() == 0 &&
		bytes.Equal(db.GetCodeHash(addr).Bytes(), gethTypes.EmptyCodeHash.Bytes())
}

// CreateAccount creates a new account for the given address
// it sets the nonce to zero
func (db *StateDB) CreateAccount(addr gethCommon.Address) {
	err := db.latestView().CreateAccount(addr)
	db.handleError(err)
}

// IsCreated returns true if address is recently created (context of a transaction)
func (db *StateDB) IsCreated(addr gethCommon.Address) bool {
	return db.latestView().IsCreated(addr)
}

// CreateContract is used whenever a contract is created. This may be preceded
// by CreateAccount, but that is not required if it already existed in the
// state due to funds sent beforehand.
// This operation sets the 'newContract'-flag, which is required in order to
// correctly handle EIP-6780 'delete-in-same-transaction' logic.
func (db *StateDB) CreateContract(addr gethCommon.Address) {
	db.latestView().CreateContract(addr)
}

// IsCreated returns true if address is a new contract
func (db *StateDB) IsNewContract(addr gethCommon.Address) bool {
	return db.latestView().IsNewContract(addr)
}

// SelfDestruct flags the address for deletion.
//
// while this address exists for the rest of transaction,
// the balance of this account is return zero after the SelfDestruct call.
func (db *StateDB) SelfDestruct(addr gethCommon.Address) {
	db.handleError(fmt.Errorf("legacy self destruct is not supported"))
}

// Selfdestruct6780 would only follow the self destruct steps if account is a new contract
// either just created, or address had balance before but got a contract deployed to it (in this tx).
func (db *StateDB) Selfdestruct6780(addr gethCommon.Address) {
	if db.IsNewContract(addr) {
		err := db.latestView().SelfDestruct(addr)
		db.handleError(err)
	}
}

// HasSelfDestructed returns true if address is flagged with self destruct.
func (db *StateDB) HasSelfDestructed(addr gethCommon.Address) bool {
	destructed, _ := db.latestView().HasSelfDestructed(addr)
	return destructed
}

// SubBalance substitutes the amount from the balance of the given address
func (db *StateDB) SubBalance(
	addr gethCommon.Address,
	amount *uint256.Int,
	reason gethTracing.BalanceChangeReason,
) {
	// negative amounts are not accepted.
	if amount.Sign() < 0 {
		db.handleError(types.ErrInvalidBalance)
		return
	}
	err := db.latestView().SubBalance(addr, amount)
	db.handleError(err)
}

// AddBalance adds the amount from the balance of the given address
func (db *StateDB) AddBalance(
	addr gethCommon.Address,
	amount *uint256.Int,
	reason gethTracing.BalanceChangeReason,
) {
	// negative amounts are not accepted.
	if amount.Sign() < 0 {
		db.handleError(types.ErrInvalidBalance)
		return
	}
	err := db.latestView().AddBalance(addr, amount)
	db.handleError(err)
}

// GetBalance returns the balance of the given address
func (db *StateDB) GetBalance(addr gethCommon.Address) *uint256.Int {
	bal, err := db.latestView().GetBalance(addr)
	db.handleError(err)
	return bal
}

// GetNonce returns the nonce of the given address
func (db *StateDB) GetNonce(addr gethCommon.Address) uint64 {
	nonce, err := db.latestView().GetNonce(addr)
	db.handleError(err)
	return nonce
}

// SetNonce sets the nonce value for the given address
func (db *StateDB) SetNonce(addr gethCommon.Address, nonce uint64) {
	err := db.latestView().SetNonce(addr, nonce)
	db.handleError(err)
}

// GetCodeHash returns the code hash of the given address
func (db *StateDB) GetCodeHash(addr gethCommon.Address) gethCommon.Hash {
	hash, err := db.latestView().GetCodeHash(addr)
	db.handleError(err)
	return hash
}

// GetCode returns the code for the given address
func (db *StateDB) GetCode(addr gethCommon.Address) []byte {
	code, err := db.latestView().GetCode(addr)
	db.handleError(err)
	return code
}

// GetCodeSize returns the size of the code for the given address
func (db *StateDB) GetCodeSize(addr gethCommon.Address) int {
	codeSize, err := db.latestView().GetCodeSize(addr)
	db.handleError(err)
	return codeSize
}

// SetCode sets the code for the given address
func (db *StateDB) SetCode(addr gethCommon.Address, code []byte) {
	err := db.latestView().SetCode(addr, code)
	db.handleError(err)
}

// AddRefund adds the amount to the total (gas) refund
func (db *StateDB) AddRefund(amount uint64) {
	err := db.latestView().AddRefund(amount)
	db.handleError(err)
}

// SubRefund subtracts the amount from the total (gas) refund
func (db *StateDB) SubRefund(amount uint64) {
	err := db.latestView().SubRefund(amount)
	db.handleError(err)
}

// GetRefund returns the total (gas) refund
func (db *StateDB) GetRefund() uint64 {
	return db.latestView().GetRefund()
}

// GetCommittedState returns the value for the given storage slot considering only the committed state and not
// changes in the scope of current transaction.
func (db *StateDB) GetCommittedState(addr gethCommon.Address, key gethCommon.Hash) gethCommon.Hash {
	value, err := db.baseView.GetState(types.SlotAddress{Address: addr, Key: key})
	db.handleError(err)
	return value
}

// GetState returns the value for the given storage slot
func (db *StateDB) GetState(addr gethCommon.Address, key gethCommon.Hash) gethCommon.Hash {
	state, err := db.latestView().GetState(types.SlotAddress{Address: addr, Key: key})
	db.handleError(err)
	return state
}

// GetStorageRoot returns some sort of root for the given address.
//
// Warning! Since StateDB doesn't construct a Merkle tree under the hood,
// the behavior of this endpoint is as follow:
// - if an account doesn't exist it returns common.Hash{}
// - if account is EOA it returns gethCommon.EmptyRootHash
// - else it returns a unique hash value as the root but this returned
//
// This behavior is ok for this version of EVM as the only
// use case in the EVM right now is here
// https://github.com/onflow/go-ethereum/blob/37590b2c5579c36d846c788c70861685b0ea240e/core/vm/evm.go#L480
// where the value that is returned is compared to empty values to make sure the storage is empty
// This endpoint is added mostly to prevent the case that an smart contract is self-destructed
// and a later transaction tries to deploy a contract to the same address.
func (db *StateDB) GetStorageRoot(addr gethCommon.Address) gethCommon.Hash {
	root, err := db.latestView().GetStorageRoot(addr)
	db.handleError(err)
	return root
}

// SetState sets a value for the given storage slot
func (db *StateDB) SetState(addr gethCommon.Address, key gethCommon.Hash, value gethCommon.Hash) {
	err := db.latestView().SetState(types.SlotAddress{Address: addr, Key: key}, value)
	db.handleError(err)
}

// GetTransientState returns the value for the given key of the transient storage
func (db *StateDB) GetTransientState(addr gethCommon.Address, key gethCommon.Hash) gethCommon.Hash {
	return db.latestView().GetTransientState(types.SlotAddress{Address: addr, Key: key})
}

// SetTransientState sets a value for the given key of the transient storage
func (db *StateDB) SetTransientState(addr gethCommon.Address, key, value gethCommon.Hash) {
	db.latestView().SetTransientState(types.SlotAddress{Address: addr, Key: key}, value)
}

// AddressInAccessList checks if an address is in the access list
func (db *StateDB) AddressInAccessList(addr gethCommon.Address) bool {
	return db.latestView().AddressInAccessList(addr)
}

// SlotInAccessList checks if the given (address,slot) is in the access list
func (db *StateDB) SlotInAccessList(addr gethCommon.Address, key gethCommon.Hash) (addressOk bool, slotOk bool) {
	return db.latestView().SlotInAccessList(types.SlotAddress{Address: addr, Key: key})
}

// AddAddressToAccessList adds the given address to the access list.
func (db *StateDB) AddAddressToAccessList(addr gethCommon.Address) {
	db.latestView().AddAddressToAccessList(addr)
}

// AddSlotToAccessList adds the given (address,slot) to the access list.
func (db *StateDB) AddSlotToAccessList(addr gethCommon.Address, key gethCommon.Hash) {
	db.latestView().AddSlotToAccessList(types.SlotAddress{Address: addr, Key: key})
}

// AddLog appends a lot to the collection of logs
func (db *StateDB) AddLog(log *gethTypes.Log) {
	db.latestView().AddLog(log)
}

// AddPreimage adds a pre-image to the collection of pre-images
func (db *StateDB) AddPreimage(hash gethCommon.Hash, data []byte) {
	db.latestView().AddPreimage(hash, data)
}

// RevertToSnapshot reverts the changes until we reach the given snapshot
func (db *StateDB) RevertToSnapshot(index int) {
	if index > len(db.views) {
		db.cachedError = fmt.Errorf("invalid revert")
		return
	}
	db.views = db.views[:index]
}

// Snapshot takes an snapshot of the state and returns an int
// that can be used later for revert calls.
func (db *StateDB) Snapshot() int {
	newView := db.latestView().NewChildView()
	db.views = append(db.views, newView)
	return len(db.views) - 1
}

// Logs returns the list of logs
// it also update each log with the block and tx info
func (db *StateDB) Logs(
	blockNumber uint64,
	txHash gethCommon.Hash,
	txIndex uint,
) []*gethTypes.Log {
	allLogs := make([]*gethTypes.Log, 0)
	for _, view := range db.views {
		for _, log := range view.Logs() {
			log.BlockNumber = blockNumber
			log.TxHash = txHash
			log.TxIndex = txIndex
			allLogs = append(allLogs, log)
		}
	}
	return allLogs
}

// Preimages returns a set of pre-images
func (db *StateDB) Preimages() map[gethCommon.Hash][]byte {
	preImages := make(map[gethCommon.Hash][]byte, 0)
	for _, view := range db.views {
		for k, v := range view.Preimages() {
			preImages[k] = v
		}
	}
	return preImages
}

// Commit commits state changes back to the underlying
func (db *StateDB) Commit(finalize bool) (hash.Hash, error) {
	// return error if any has been accumulated
	if db.cachedError != nil {
		return nil, wrapError(db.cachedError)
	}

	var err error

	// iterate views and collect dirty addresses and slots
	addresses := make(map[gethCommon.Address]struct{})
	slots := make(map[types.SlotAddress]struct{})
	for _, view := range db.views {
		for key := range view.DirtyAddresses() {
			addresses[key] = struct{}{}
		}
		for key := range view.DirtySlots() {
			slots[key] = struct{}{}
		}
	}

	// sort addresses
	sortedAddresses := make([]gethCommon.Address, 0, len(addresses))
	for addr := range addresses {
		sortedAddresses = append(sortedAddresses, addr)
	}

	sort.Slice(sortedAddresses,
		func(i, j int) bool {
			return bytes.Compare(sortedAddresses[i][:], sortedAddresses[j][:]) < 0
		})

	updateCommitter := NewUpdateCommitter()
	// update accounts
	for _, addr := range sortedAddresses {
		deleted := false
		// first we need to delete accounts
		if db.HasSelfDestructed(addr) {
			err = db.baseView.DeleteAccount(addr)
			if err != nil {
				return nil, wrapError(err)
			}
			err = updateCommitter.DeleteAccount(addr)
			if err != nil {
				return nil, wrapError(err)
			}
			deleted = true
		}
		if deleted {
			continue
		}

		bal := db.GetBalance(addr)
		nonce := db.GetNonce(addr)
		code := db.GetCode(addr)
		codeHash := db.GetCodeHash(addr)
		// create new accounts
		if db.IsCreated(addr) {
			err = db.baseView.CreateAccount(
				addr,
				bal,
				nonce,
				code,
				codeHash,
			)
			if err != nil {
				return nil, wrapError(err)
			}
			err = updateCommitter.CreateAccount(addr, bal, nonce, codeHash)
			if err != nil {
				return nil, wrapError(err)
			}
			continue
		}
		err = db.baseView.UpdateAccount(
			addr,
			bal,
			nonce,
			code,
			codeHash,
		)
		if err != nil {
			return nil, wrapError(err)
		}
		err = updateCommitter.UpdateAccount(addr, bal, nonce, codeHash)
		if err != nil {
			return nil, wrapError(err)
		}
	}

	// sort slots
	sortedSlots := make([]types.SlotAddress, 0, len(slots))
	for slot := range slots {
		sortedSlots = append(sortedSlots, slot)
	}
	sort.Slice(sortedSlots, func(i, j int) bool {
		comp := bytes.Compare(sortedSlots[i].Address[:], sortedSlots[j].Address[:])
		if comp == 0 {
			return bytes.Compare(sortedSlots[i].Key[:], sortedSlots[j].Key[:]) < 0
		}
		return comp < 0
	})

	// update slots
	for _, sk := range sortedSlots {
		// don't update slots if self destructed
		if db.HasSelfDestructed(sk.Address) {
			continue
		}
		val := db.GetState(sk.Address, sk.Key)
		err = db.baseView.UpdateSlot(
			sk,
			val,
		)
		if err != nil {
			return nil, wrapError(err)
		}
		err = updateCommitter.UpdateSlot(sk.Address, sk.Key, val)
		if err != nil {
			return nil, wrapError(err)
		}
	}

	// don't purge views yet, people might call the logs etc
	updateCommit := updateCommitter.Commitment()
	if finalize {
		err := db.Finalize()
		if err != nil {
			return nil, err
		}
	}
	return updateCommit, nil
}

// Finalize flushes all the changes
// to the permanent storage
func (db *StateDB) Finalize() error {
	err := db.baseView.Commit()
	return wrapError(err)
}

// Prepare is a high level logic that sadly is considered to be part of the
// stateDB interface and not on the layers above.
// based on parameters that are passed it updates access-lists
func (db *StateDB) Prepare(rules gethParams.Rules, sender, coinbase gethCommon.Address, dest *gethCommon.Address, precompiles []gethCommon.Address, txAccesses gethTypes.AccessList) {
	if rules.IsBerlin {
		db.AddAddressToAccessList(sender)

		if dest != nil {
			db.AddAddressToAccessList(*dest)
			// If it's a create-tx, the destination will be added inside egethVM.create
		}
		for _, addr := range precompiles {
			db.AddAddressToAccessList(addr)
		}
		for _, el := range txAccesses {
			db.AddAddressToAccessList(el.Address)
			for _, key := range el.StorageKeys {
				db.AddSlotToAccessList(el.Address, key)
			}
		}
		if rules.IsShanghai { // EIP-3651: warm coinbase
			db.AddAddressToAccessList(coinbase)
		}
	}
}

// Reset resets uncommitted changes and transient artifacts such as error, logs,
// pre-images, access lists, ...
// The method is often called between execution of different transactions
func (db *StateDB) Reset() {
	db.views = []*DeltaView{NewDeltaView(db.baseView)}
	db.cachedError = nil
}

// Error returns the memorized database failure occurred earlier.
func (s *StateDB) Error() error {
	return wrapError(s.cachedError)
}

// PointCache is not supported and only needed
// when EIP-4762 is enabled in the future versions
// (currently planned for after Verkle fork).
func (s *StateDB) PointCache() *gethUtils.PointCache {
	return nil
}

// Witness is not supported and only needed
// when if witness collection is enabled (EnableWitnessCollection flag).
// By definition it should returns a set containing all trie nodes that have been accessed.
// The returned map could be nil if the witness is empty.
func (s *StateDB) Witness() *gethStateless.Witness {
	return nil
}

func (db *StateDB) latestView() *DeltaView {
	return db.views[len(db.views)-1]
}

// set error captures the first non-nil error it is called with.
func (db *StateDB) handleError(err error) {
	if err == nil {
		return
	}
	if db.cachedError == nil {
		db.cachedError = err
	}
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}

	var atreeUserError *atree.UserError
	// if is an atree user error
	if stdErrors.As(err, &atreeUserError) {
		return types.NewStateError(err)
	}

	var atreeFatalError *atree.FatalError
	// if is a atree fatal error or
	if stdErrors.As(err, &atreeFatalError) {
		return types.NewFatalError(err)
	}

	// if is a fatal error
	if types.IsAFatalError(err) {
		return err
	}

	return types.NewStateError(err)
}
