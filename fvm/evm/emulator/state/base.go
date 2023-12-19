package state

import (
	"fmt"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// the path where we store the collection ID for accounts
	AccountsStorageIDKey = "AccountsStorageIDKey"
	// the path where we store the collection ID for codes
	CodesStorageIDKey = "CodesStorageIDKey"
)

// BaseView implements a types.BaseView
// it acts as the base layer of state queries for the stateDB
// it stores accounts, codes and storage slots.
//
// under the hood it uses a set of collections,
// one for account's meta data, one for codes
// and one for each of account storage space.
type BaseView struct {
	rootAddress        flow.Address
	ledger             atree.Ledger
	collectionProvider *CollectionProvider

	// collections
	accounts *Collection
	codes    *Collection
	slots    map[gethCommon.Address]*Collection

	// cached values
	cachedAccounts map[gethCommon.Address]*Account
	cachedCodes    map[gethCommon.Address][]byte
	cachedSlots    map[types.SlotAddress]gethCommon.Hash

	// flags
	accountSetupOnCommit bool
	codeSetupOnCommit    bool
}

var _ types.BaseView = &BaseView{}

// NewBaseView constructs a new base view
func NewBaseView(ledger atree.Ledger, rootAddress flow.Address) (*BaseView, error) {
	cp, err := NewCollectionProvider(atree.Address(rootAddress), ledger)
	if err != nil {
		return nil, err
	}

	view := &BaseView{
		ledger:             ledger,
		rootAddress:        rootAddress,
		collectionProvider: cp,

		slots: make(map[gethCommon.Address]*Collection),

		cachedAccounts: make(map[gethCommon.Address]*Account),
		cachedCodes:    make(map[gethCommon.Address][]byte),
		cachedSlots:    make(map[types.SlotAddress]gethCommon.Hash),
	}

	// fetch the account collection, if not exist, create one
	view.accounts, view.accountSetupOnCommit, err = view.fetchOrCreateCollection(AccountsStorageIDKey)
	if err != nil {
		return nil, err
	}

	// fetch the code collection, if not exist, create one
	view.codes, view.codeSetupOnCommit, err = view.fetchOrCreateCollection(CodesStorageIDKey)
	if err != nil {
		return nil, err
	}

	return view, nil
}

// Exist returns true if the address exist in the state
func (v *BaseView) Exist(addr gethCommon.Address) (bool, error) {
	acc, err := v.getAccount(addr)
	return acc != nil, err
}

// IsCreated returns true if the address has been created in the context of this transaction
func (v *BaseView) IsCreated(gethCommon.Address) bool {
	return false
}

// HasSuicided returns true if an address has suicided in the context of this transaction
func (v *BaseView) HasSuicided(gethCommon.Address) bool {
	return false
}

// GetBalance returns the balance of an address
//
// for non-existent accounts it returns a balance of zero
func (v *BaseView) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	acc, err := v.getAccount(addr)
	bal := big.NewInt(0)
	if acc != nil {
		bal = acc.Balance
	}
	return bal, err
}

// GetNonce returns the nonce of an address
//
// for non-existent accounts it returns zero
func (v *BaseView) GetNonce(addr gethCommon.Address) (uint64, error) {
	acc, err := v.getAccount(addr)
	nonce := uint64(0)
	if acc != nil {
		nonce = acc.Nonce
	}
	return nonce, err
}

// GetCode returns the code of an address
//
// for non-existent accounts or accounts without a code (e.g. EOAs) it returns nil
func (v *BaseView) GetCode(addr gethCommon.Address) ([]byte, error) {
	return v.getCode(addr)
}

// GetCodeHash returns the code hash of an address
//
// for non-existent accounts or accounts without a code (e.g. EOAs) it returns default empty
// hash value (gethTypes.EmptyCodeHash)
func (v *BaseView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	acc, err := v.getAccount(addr)
	codeHash := gethTypes.EmptyCodeHash
	if acc != nil {
		codeHash = acc.CodeHash
	}
	return codeHash, err
}

// GetCodeSize returns the code size of an address
//
// for non-existent accounts or accounts without a code (e.g. EOAs) it returns zero
func (v *BaseView) GetCodeSize(addr gethCommon.Address) (int, error) {
	code, err := v.GetCode(addr)
	return len(code), err
}

// GetState returns values for an slot in the main storage
//
// for non-existent slots it returns the default empty hash value (gethTypes.EmptyCodeHash)
func (v *BaseView) GetState(sk types.SlotAddress) (gethCommon.Hash, error) {
	return v.getSlot(sk)
}

// UpdateSlot updates the value for a slot
func (v *BaseView) UpdateSlot(sk types.SlotAddress, value gethCommon.Hash) error {
	return v.storeSlot(sk, value)
}

// GetRefund returns the total amount of (gas) refund
//
// this method returns the value of zero
func (v *BaseView) GetRefund() uint64 {
	return 0
}

// GetTransientState returns values for an slot transient storage
//
// transient storage is not a functionality for the base view so it always
// returns the default value for non-existent slots
func (v *BaseView) GetTransientState(types.SlotAddress) gethCommon.Hash {
	return gethCommon.Hash{}
}

// AddressInAccessList checks if an address is in the access list
//
// access list control is not a functionality of the base view
// it always returns false
func (v *BaseView) AddressInAccessList(gethCommon.Address) bool {
	return false
}

// SlotInAccessList checks if a slot is in the access list
//
// access list control is not a functionality of the base view
// it always returns false
func (v *BaseView) SlotInAccessList(types.SlotAddress) (addressOk bool, slotOk bool) {
	return false, false
}

// CreateAccount creates a new account
func (v *BaseView) CreateAccount(
	addr gethCommon.Address,
	balance *big.Int,
	nonce uint64,
	code []byte,
	codeHash gethCommon.Hash,
) error {
	var colID []byte
	// if is an smart contract account
	if len(code) > 0 {
		err := v.storeCode(addr, code)
		if err != nil {
			return err
		}

		// create a new collection for slots
		col, err := v.collectionProvider.NewCollection()
		if err != nil {
			return err
		}
		colID = col.CollectionID()
	}

	// create a new account and store it
	acc := NewAccount(addr, balance, nonce, codeHash, colID)

	// no need to update the cache , storeAccount would update the cache
	return v.storeAccount(acc)
}

// UpdateAccount updates an account's meta data
func (v *BaseView) UpdateAccount(
	addr gethCommon.Address,
	balance *big.Int,
	nonce uint64,
	code []byte,
	codeHash gethCommon.Hash,
) error {
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	// if update is called on a non existing account
	// we gracefully call the create account
	// TODO: but we might need to revisit this action in the future
	if acc == nil {
		return v.CreateAccount(addr, balance, nonce, code, codeHash)
	}
	// if it has a code change
	if codeHash != acc.CodeHash {
		err := v.storeCode(addr, code)
		if err != nil {
			return err
		}
		// TODO: maybe purge the state in the future as well
		// currently the behaviour of stateDB doesn't purge the data
	}
	newAcc := NewAccount(addr, balance, nonce, codeHash, acc.CollectionID)
	// no need to update the cache , storeAccount would update the cache
	return v.storeAccount(newAcc)
}

// DeleteAccount deletes an account's meta data, code, and
// storage slots associated with that address
func (v *BaseView) DeleteAccount(addr gethCommon.Address) error {
	// 1. check account exists
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("account doesn't exist to be deleted")
	}

	// 2. update the cache
	delete(v.cachedAccounts, addr)

	// 3. collections
	err = v.accounts.Remove(addr.Bytes())
	if err != nil {
		return err
	}

	// 4. remove the code
	if acc.HasCode() {
		err = v.storeCode(addr, nil)
		if err != nil {
			return err
		}
	}

	// 5. remove storage slots
	if len(acc.CollectionID) > 0 {
		col, found := v.slots[addr]
		if !found {
			col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
			if err != nil {
				return err
			}
		}
		// delete all slots related to this account (eip-6780)
		err = col.Destroy()
		if err != nil {
			return err
		}
	}
	return nil
}

// Commit commits the changes to the underlying storage layers
func (v *BaseView) Commit() error {
	// commit collection changes
	err := v.collectionProvider.Commit()
	if err != nil {
		return err
	}

	// if this is the first time we are setting up an
	// account collection, store its collection id.
	if v.accountSetupOnCommit {
		err = v.ledger.SetValue(v.rootAddress[:], []byte(AccountsStorageIDKey), v.accounts.CollectionID())
		if err != nil {
			return err
		}
		v.accountSetupOnCommit = false

	}

	// if this is the first time we are setting up an
	// code collection, store its collection id.
	if v.codeSetupOnCommit {
		err = v.ledger.SetValue(v.rootAddress[:], []byte(CodesStorageIDKey), v.codes.CollectionID())
		if err != nil {
			return err
		}
		v.codeSetupOnCommit = false
	}
	return nil
}

func (v *BaseView) fetchOrCreateCollection(path string) (collection *Collection, created bool, error error) {
	collectionID, err := v.ledger.GetValue(v.rootAddress[:], []byte(path))
	if err != nil {
		return nil, false, err
	}
	if len(collectionID) == 0 {
		collection, err = v.collectionProvider.NewCollection()
		return collection, true, err
	}
	collection, err = v.collectionProvider.CollectionByID(collectionID)
	return collection, false, err
}

func (v *BaseView) getAccount(addr gethCommon.Address) (*Account, error) {
	// check cached accounts first
	acc, found := v.cachedAccounts[addr]
	if found {
		return acc, nil
	}

	// then collect it from the account collection
	data, err := v.accounts.Get(addr.Bytes())
	if err != nil {
		return nil, err
	}
	// decode it
	acc, err = DecodeAccount(data)
	if err != nil {
		return nil, err
	}
	// cache it
	if acc != nil {
		v.cachedAccounts[addr] = acc
	}
	return acc, nil
}

func (v *BaseView) storeAccount(acc *Account) error {
	data, err := acc.Encode()
	if err != nil {
		return err
	}
	// update the cache
	v.cachedAccounts[acc.Address] = acc
	return v.accounts.Set(acc.Address.Bytes(), data)
}

func (v *BaseView) getCode(addr gethCommon.Address) ([]byte, error) {
	// check the cache first
	code, found := v.cachedCodes[addr]
	if found {
		return code, nil
	}
	// check if account exist and has codeHash
	acc, err := v.getAccount(addr)
	if err != nil {
		return nil, err
	}
	// if no account found return
	if acc == nil {
		return nil, nil
	}
	// check account has code
	if !acc.HasCode() {
		return nil, nil
	}
	// then collect it from the code collection
	code, err = v.codes.Get(addr.Bytes())
	if err != nil {
		return nil, err
	}
	if code != nil {
		v.cachedCodes[addr] = code
	}
	return code, nil
}

func (v *BaseView) storeCode(addr gethCommon.Address, code []byte) error {
	if len(code) == 0 {
		delete(v.cachedCodes, addr)
		return v.codes.Remove(addr.Bytes())
	}
	v.cachedCodes[addr] = code
	return v.codes.Set(addr.Bytes(), code)
}

func (v *BaseView) getSlot(sk types.SlotAddress) (gethCommon.Hash, error) {
	value, found := v.cachedSlots[sk]
	if found {
		return value, nil
	}

	col, err := v.getSlotCollection(sk.Address)
	if err != nil {
		return gethCommon.Hash{}, err
	}

	val, err := col.Get(sk.Key.Bytes())
	if err != nil {
		return gethCommon.Hash{}, err
	}
	value = gethCommon.BytesToHash(val)
	v.cachedSlots[sk] = value
	return value, nil
}

func (v *BaseView) storeSlot(sk types.SlotAddress, data gethCommon.Hash) error {
	col, err := v.getSlotCollection(sk.Address)
	if err != nil {
		return err
	}
	v.cachedSlots[sk] = data
	return col.Set(sk.Key.Bytes(), data.Bytes())
}

func (v *BaseView) getSlotCollection(addr gethCommon.Address) (*Collection, error) {
	acc, err := v.getAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, fmt.Errorf("slot belongs to a non-existing account")
	}
	if len(acc.CollectionID) == 0 {
		return nil, fmt.Errorf("slot belongs to a non-smart contract account")
	}
	col, found := v.slots[acc.Address]
	if !found {
		col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
		if err != nil {
			return nil, err
		}
		v.slots[acc.Address] = col
	}
	return col, err
}
