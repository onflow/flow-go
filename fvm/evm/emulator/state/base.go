package state

import (
	stdErrors "errors"
	"fmt"
	"math/big"
	"runtime"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// TODO build a not performant version of storage (commited view), we need to update
// issue with caching for spocks (this view has to be constructed for each transaction)
// the internal db can be a reused.

const (
	AccountsStorageIDKey = "AccountsStorageIDKey"
	CodesStorageIDKey    = "CodesStorageIDKey"
	StorageIDSize        = 16
)

// TODO rename to BaseView
type BaseView struct {
	rootAddress flow.Address
	ledger      atree.Ledger
	baseStorage *atree.LedgerBaseStorage
	storage     *atree.PersistentSlabStorage

	// singular maps
	accounts *atree.OrderedMap
	codes    *atree.OrderedMap

	// caches
	cachedAccounts map[gethCommon.Address]*account
	cachedCodes    map[gethCommon.Address][]byte
	cachedSlots    map[types.SlotAddress]gethCommon.Hash

	// flags
	accountSetupOnCommit bool
	codeSetupOnCommit    bool
}

// NewBaseView constructs a new base view
func NewBaseView(ledger atree.Ledger, rootAddress flow.Address) (*BaseView, error) {
	view := &BaseView{
		ledger:      ledger,
		rootAddress: rootAddress,
		baseStorage: atree.NewLedgerBaseStorage(ledger),

		cachedAccounts: make(map[gethCommon.Address]*account),
		cachedCodes:    make(map[gethCommon.Address][]byte),
		cachedSlots:    make(map[types.SlotAddress]gethCommon.Hash),
	}

	var err error
	view.storage, err = NewPersistentSlabStorage(view.baseStorage)
	if err != nil {
		return nil, handleError(err)
	}

	view.accounts, view.accountSetupOnCommit, err = view.retrieveOrCreateMap(AccountsStorageIDKey)
	if err != nil {
		return nil, handleError(err)
	}

	view.codes, view.codeSetupOnCommit, err = view.retrieveOrCreateMap(CodesStorageIDKey)
	if err != nil {
		return nil, handleError(err)
	}

	return view, nil
}

var _ types.BaseView = &BaseView{}

func (v *BaseView) Exist(addr gethCommon.Address) (bool, error) {
	acc, err := v.getAccount(addr)
	return acc != nil, handleError(err)
}

func (v *BaseView) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	acc, err := v.getAccount(addr)
	bal := big.NewInt(0)
	if acc != nil {
		bal = acc.balance
	}
	return bal, handleError(err)
}

func (v *BaseView) GetNonce(addr gethCommon.Address) (uint64, error) {
	acc, err := v.getAccount(addr)
	nonce := uint64(0)
	if acc != nil {
		nonce = acc.nonce
	}
	return nonce, handleError(err)
}

func (v *BaseView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	acc, err := v.getAccount(addr)
	codeHash := gethTypes.EmptyCodeHash
	if acc != nil {
		codeHash = acc.codeHash
	}
	return codeHash, handleError(err)
}

func (v *BaseView) GetCode(addr gethCommon.Address) ([]byte, error) {
	// check the codeHash first
	acc, err := v.getAccount(addr)
	// if no account found return
	if acc != nil {
		return nil, nil
	}
	// no code on this account
	if acc.codeHash == gethTypes.EmptyCodeHash {
		return nil, nil
	}

	code, err := v.getCode(addr)
	return code, handleError(err)
}

func (v *BaseView) GetCodeSize(addr gethCommon.Address) (int, error) {
	// check the codeHash first
	acc, err := v.getAccount(addr)
	// if no account found return
	if acc != nil {
		return 0, nil
	}
	// no code on this account
	if acc.codeHash == gethTypes.EmptyCodeHash {
		return 0, nil
	}

	code, err := v.getCode(addr)
	return len(code), handleError(err)
}

func (v *BaseView) GetState(sk types.SlotAddress) (gethCommon.Hash, error) {
	return v.getSlot(sk)
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

func (v *BaseView) updateAccount(acc *account) error {
	return v.storeAccount(acc)
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

		// create storage map
		accountStateMap, err := atree.NewMap(
			v.storage,
			atree.Address(v.rootAddress),
			atree.NewDefaultDigesterBuilder(),
			emptyTypeInfo{},
		)
		if err != nil {
			return err
		}
		sID, err = storageIDToBytes(accountStateMap.StorageID())
		if err != nil {
			return err
		}
	}

	acc := newAccount(addr, balance, nonce, codeHash, sID)
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
	// if code change
	if codeHash != acc.codeHash {
		err := v.storeCode(addr, code)
		if err != nil {
			return err
		}
		// TODO: maybe purge the state as well
	}
	newAcc := newAccount(addr, balance, nonce, codeHash, acc.storageIDBytes)
	return v.storeAccount(newAcc)
}

func (v *BaseView) DeleteAccount(addr gethCommon.Address) error {
	// TODO implement me

	// We need a way to also delete all slots related to this account (eip-6780)
	return nil
}

func (v *BaseView) UpdateSlot(sk types.SlotAddress, value gethCommon.Hash) error {
	return v.storeSlot(sk, value)
}

func (v *BaseView) Commit() error {
	// commit atree changes
	err := v.storage.FastCommit(runtime.NumCPU())
	if err != nil {
		return err
	}

	if v.accountSetupOnCommit {
		data, err := storageIDToBytes(v.accounts.StorageID())
		if err != nil {
			return handleError(err)
		}
		err = v.ledger.SetValue(v.rootAddress[:], []byte(AccountsStorageIDKey), data)
		if err != nil {
			return handleError(err)
		}
	}

	if v.codeSetupOnCommit {
		data, err := storageIDToBytes(v.codes.StorageID())
		if err != nil {
			return handleError(err)
		}
		err = v.ledger.SetValue(v.rootAddress[:], []byte(CodesStorageIDKey), data)
		if err != nil {
			return handleError(err)
		}
	}
	return nil
}

func (v *BaseView) retrieveOrCreateMap(path string) (omap *atree.OrderedMap, requireSetup bool, err error) {
	storageIDBytes, err := v.ledger.GetValue(v.rootAddress.Bytes(), []byte(path))
	if err != nil {
		return nil, false, err
	}

	// if map doesn't exist
	if len(storageIDBytes) == 0 {
		m, err := atree.NewMap(v.storage, atree.Address(v.rootAddress), atree.NewDefaultDigesterBuilder(), emptyTypeInfo{})
		return m, true, err
	}

	storageID, err := atree.NewStorageIDFromRawBytes(storageIDBytes)
	if err != nil {
		return nil, false, err
	}
	m, err := atree.NewMapWithRootID(v.storage, storageID, atree.NewDefaultDigesterBuilder())
	return m, false, err
}

func (v *BaseView) getAccount(addr gethCommon.Address) (*account, error) {
	acc, found := v.cachedAccounts[addr]
	if found {
		return acc, nil
	}
	acc, err := v.fetchAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		v.cachedAccounts[addr] = acc
	}
	return acc, nil
}

func (v *BaseView) fetchAccount(addr gethCommon.Address) (*account, error) {
	// first check if we have the address in the accounts
	// this escapes hitting NotFound errors
	found, err := v.accounts.Has(compare, hashInputProvider, NewByteStringValue(addr.Bytes()))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	// then get account
	data, err := v.accounts.Get(compare, hashInputProvider, NewByteStringValue(addr.Bytes()))
	if err != nil {
		return nil, err
	}

	value, err := data.StoredValue(v.accounts.Storage)
	if err != nil {
		return nil, err
	}

	bytes := value.(ByteStringValue).Bytes()
	return decodeAccount(bytes)
}

func (v *BaseView) storeAccount(acc *account) error {
	data, err := acc.encode()
	if err != nil {
		return err
	}

	existingValueStorable, err := v.accounts.Set(compare, hashInputProvider, NewByteStringValue(acc.address.Bytes()), NewByteStringValue(data))
	if err != nil {
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := v.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *BaseView) getCode(addr gethCommon.Address) ([]byte, error) {
	code, found := v.cachedCodes[addr]
	if found {
		return code, nil
	}
	code, err := v.fetchCode(addr)
	if err != nil {
		return nil, err
	}
	if code != nil {
		v.cachedCodes[addr] = code
	}
	return code, nil
}

func (v *BaseView) fetchCode(addr gethCommon.Address) ([]byte, error) {
	// first check if we have the address in the codes
	// this escapes hitting NotFound errors
	found, err := v.codes.Has(compare, hashInputProvider, NewByteStringValue(addr.Bytes()))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	// then get the code
	data, err := v.codes.Get(compare, hashInputProvider, NewByteStringValue(addr.Bytes()))
	if err != nil {
		return nil, err
	}

	value, err := data.StoredValue(v.accounts.Storage)
	return value.(ByteStringValue).Bytes(), err
}

func (v *BaseView) storeCode(addr gethCommon.Address, code []byte) error {
	existingValueStorable, err := v.accounts.Set(compare, hashInputProvider, NewByteStringValue(addr.Bytes()), NewByteStringValue(code))
	if err != nil {
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := v.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *BaseView) getSlot(sk types.SlotAddress) (gethCommon.Hash, error) {
	defValue := gethCommon.Hash{}
	value, found := v.cachedSlots[sk]
	if found {
		return value, nil
	}
	value, err := v.fetchSlot(sk)
	if err != nil {
		return defValue, err
	}

	if value != defValue {
		v.cachedSlots[sk] = value
	}
	return value, nil
}

func (v *BaseView) fetchSlot(sk types.SlotAddress) (gethCommon.Hash, error) {
	// get account first
	acc, err := v.getAccount(sk.Address)
	if err != nil || acc == nil || len(acc.storageIDBytes) == 0 {
		return gethCommon.Hash{}, err
	}

	storageID, err := atree.NewStorageIDFromRawBytes(acc.storageIDBytes)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	accountStateMap, err := atree.NewMapWithRootID(v.storage, storageID, atree.NewDefaultDigesterBuilder())

	found, err := accountStateMap.Has(compare, hashInputProvider, NewByteStringValue(sk.Key.Bytes()))
	if err != nil || !found {
		return gethCommon.Hash{}, err
	}

	data, err := accountStateMap.Get(compare, hashInputProvider, NewByteStringValue(sk.Key.Bytes()))
	if err != nil {
		return gethCommon.Hash{}, nil
	}

	value, err := data.StoredValue(accountStateMap.Storage)
	return gethCommon.BytesToHash(value.(ByteStringValue).Bytes()), err
}

func (v *BaseView) storeSlot(sk types.SlotAddress, data gethCommon.Hash) error {
	acc, err := v.getAccount(sk.Address)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("account doesn't exist")
	}

	var accountStateMap *atree.OrderedMap
	if len(acc.storageIDBytes) == 0 {
		return fmt.Errorf("storageIDBytes is empty but it shouldn't be")

	} else {
		storageID, err := atree.NewStorageIDFromRawBytes(acc.storageIDBytes)
		if err != nil {
			return err
		}
		accountStateMap, err = atree.NewMapWithRootID(v.storage, storageID, atree.NewDefaultDigesterBuilder())
		if err != nil {
			return err
		}
	}

	// TODO: if value is common.Hash remove the item from the list
	existingValueStorable, err := accountStateMap.Set(
		compare,
		hashInputProvider,
		NewByteStringValue(acc.address.Bytes()),
		NewByteStringValue(data.Bytes()),
	)
	if err != nil {
		return err
	}

	if id, ok := existingValueStorable.(atree.StorageIDStorable); ok {
		// NOTE: deep remove isn't necessary because value is ByteStringValue (not container)
		err := v.storage.Remove(atree.StorageID(id))
		if err != nil {
			return err
		}
	}
	return nil
}

func handleError(err error) error {
	if err == nil {
		return nil
	}

	var atreeFatalError *atree.FatalError
	// if is a atree fatal error or fvm fatal error (the second one captures external errors)
	if stdErrors.As(err, &atreeFatalError) || errors.IsFailure(err) {
		return types.NewFatalError(err)
	}
	// wrap the non-fatal error with DB error
	return types.NewDatabaseError(err)
}

func storageIDToBytes(sID atree.StorageID) ([]byte, error) {
	data := make([]byte, StorageIDSize)
	_, err := sID.ToRawBytes(data)
	return data, err
}
