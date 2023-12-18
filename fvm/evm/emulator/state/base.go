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

// TODO we need to handle the
// issue with caching for spocks (this view has to be constructed for each transaction)
// the internal db can be a reused.

const (
	AccountsStorageIDKey = "AccountsStorageIDKey"
	CodesStorageIDKey    = "CodesStorageIDKey"
	StorageIDSize        = 16
)

type BaseView struct {
	rootAddress        flow.Address
	ledger             atree.Ledger
	collectionProvider *CollectionProvider

	// collections
	accounts *Collection
	codes    *Collection
	slots    map[gethCommon.Address]*Collection

	// caches
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

	view.accounts, view.accountSetupOnCommit, err = view.fetchOrCreateCollection(AccountsStorageIDKey)
	if err != nil {
		return nil, err
	}

	view.codes, view.codeSetupOnCommit, err = view.fetchOrCreateCollection(CodesStorageIDKey)
	if err != nil {
		return nil, err
	}

	return view, nil
}

func (v *BaseView) Exist(addr gethCommon.Address) (bool, error) {
	acc, err := v.getAccount(addr)
	return acc != nil, err
}

func (v *BaseView) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	acc, err := v.getAccount(addr)
	bal := big.NewInt(0)
	if acc != nil {
		bal = acc.Balance
	}
	return bal, err
}

func (v *BaseView) GetNonce(addr gethCommon.Address) (uint64, error) {
	acc, err := v.getAccount(addr)
	nonce := uint64(0)
	if acc != nil {
		nonce = acc.Nonce
	}
	return nonce, err
}

func (v *BaseView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	acc, err := v.getAccount(addr)
	codeHash := gethTypes.EmptyCodeHash
	if acc != nil {
		codeHash = acc.CodeHash
	}
	return codeHash, err
}

func (v *BaseView) GetCode(addr gethCommon.Address) ([]byte, error) {
	// check the codeHash first
	acc, err := v.getAccount(addr)
	if err != nil {
		return nil, err
	}
	// if no account found return
	if acc == nil {
		return nil, nil
	}
	// if no code on this account
	if acc.CodeHash == gethTypes.EmptyCodeHash {
		return nil, nil
	}

	code, err := v.getCode(addr)
	return code, err
}

func (v *BaseView) GetCodeSize(addr gethCommon.Address) (int, error) {
	// check the codeHash first
	acc, err := v.getAccount(addr)
	if err != nil {
		return 0, err
	}
	// if no account found return
	if acc == nil {
		return 0, nil
	}
	// no code on this account
	if acc.CodeHash == gethTypes.EmptyCodeHash {
		return 0, nil
	}

	code, err := v.getCode(addr)
	return len(code), err
}

func (v *BaseView) GetState(sk types.SlotAddress) (gethCommon.Hash, error) {
	return v.getSlot(sk)
}

func (v *BaseView) UpdateSlot(sk types.SlotAddress, value gethCommon.Hash) error {
	return v.storeSlot(sk, value)
}

func (v *BaseView) HasSuicided(gethCommon.Address) bool {
	return false
}

func (v *BaseView) GetRefund() uint64 {
	return 0
}

func (v *BaseView) GetTransientState(types.SlotAddress) gethCommon.Hash {
	return gethCommon.Hash{}
}

func (v *BaseView) AddressInAccessList(gethCommon.Address) bool {
	return false
}

func (v *BaseView) SlotInAccessList(types.SlotAddress) (addressOk bool, slotOk bool) {
	return false, false
}

func (v *BaseView) Commit() error {
	// commit collections
	err := v.collectionProvider.Commit()
	if err != nil {
		return err
	}
	if v.accountSetupOnCommit {
		err = v.ledger.SetValue(v.rootAddress[:], []byte(AccountsStorageIDKey), v.accounts.CollectionID())
		if err != nil {
			return err
		}
		v.accountSetupOnCommit = false

	}

	if v.codeSetupOnCommit {
		err = v.ledger.SetValue(v.rootAddress[:], []byte(CodesStorageIDKey), v.codes.CollectionID())
		if err != nil {
			return err
		}
		v.codeSetupOnCommit = false
	}
	return nil
}

func (v *BaseView) PurgeCaches() {
	v.cachedAccounts = make(map[gethCommon.Address]*Account)
	v.cachedCodes = make(map[gethCommon.Address][]byte)
	v.cachedSlots = make(map[types.SlotAddress]gethCommon.Hash)
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

func (v *BaseView) CreateAccount(
	addr gethCommon.Address,
	balance *big.Int,
	nonce uint64,
	code []byte,
	codeHash gethCommon.Hash,
) error {
	var sID []byte
	if len(code) > 0 {
		err := v.storeCode(addr, code)
		if err != nil {
			return err
		}

		col, err := v.collectionProvider.NewCollection()
		if err != nil {
			return err
		}
		sID = col.CollectionID()
	}

	acc := NewAccount(addr, balance, nonce, codeHash, sID)
	return v.storeAccount(acc)
}

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
	// if it has a code change
	if codeHash != acc.CodeHash {
		err := v.storeCode(addr, code)
		if err != nil {
			return err
		}
		// TODO: maybe purge the state as well
	}
	newAcc := NewAccount(addr, balance, nonce, codeHash, acc.CollectionID)
	return v.storeAccount(newAcc)
}

func (v *BaseView) DeleteAccount(addr gethCommon.Address) error {
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	if acc != nil {
		return fmt.Errorf("account doesn't exist")
	}

	err = v.accounts.Remove(addr.Bytes())
	if err != nil {
		return err
	}

	delete(v.cachedAccounts, addr)

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

func (v *BaseView) storeAccount(acc *Account) error {
	data, err := acc.Encode()
	if err != nil {
		return err
	}
	v.cachedAccounts[acc.Address] = acc
	return v.accounts.Set(acc.Address.Bytes(), data)
}

func (v *BaseView) getCode(addr gethCommon.Address) ([]byte, error) {
	code, found := v.cachedCodes[addr]
	if found {
		return code, nil
	}
	code, err := v.codes.Get(addr.Bytes())
	if err != nil {
		return nil, err
	}
	if code != nil {
		v.cachedCodes[addr] = code
	}
	return code, nil
}

func (v *BaseView) storeCode(addr gethCommon.Address, code []byte) error {
	v.cachedCodes[addr] = code
	return v.codes.Set(addr.Bytes(), code)
}

func (v *BaseView) getSlot(sk types.SlotAddress) (gethCommon.Hash, error) {
	value, found := v.cachedSlots[sk]
	if found {
		return value, nil
	}

	// check account
	acc, err := v.getAccount(sk.Address)
	if err != nil || acc == nil || len(acc.CollectionID) == 0 {
		return gethCommon.Hash{}, err
	}

	col, found := v.slots[sk.Address]
	if !found {
		col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
		if err != nil {
			return gethCommon.Hash{}, err
		}
		v.slots[sk.Address] = col
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
	acc, err := v.getAccount(sk.Address)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("slot belongs to a non existing account")
	}
	if len(acc.CollectionID) == 0 {
		return fmt.Errorf("slot belongs to a non-smart contract account")
	}

	col, found := v.slots[sk.Address]
	if !found {
		col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
		if err != nil {
			return err
		}
		v.slots[sk.Address] = col
	}

	v.cachedSlots[sk] = data
	return col.Set(sk.Key.Bytes(), data.Bytes())
}
