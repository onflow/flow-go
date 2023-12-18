package state

import (
	"bytes"
	stdErrors "errors"
	"fmt"
	"math/big"
	"sort"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// TODO: question, does stateDB has to be thread safe ?
// error handling
type StateDB struct {
	ledger   atree.Ledger
	root     flow.Address
	baseView types.BaseView
	views    []*DeltaView
	dbErr    error
}

var _ types.StateDB = &StateDB{}

func NewStateDB(ledger atree.Ledger, root flow.Address) (*StateDB, error) {
	bv, err := NewBaseView(ledger, root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		ledger:   ledger,
		root:     root,
		baseView: bv,
		views:    []*DeltaView{NewDeltaView(bv)},
		dbErr:    nil,
	}, nil
}

// Exist returns true if the given address exists in state.
//
// this should also return true for suicided accounts during the transaction execution.
func (db *StateDB) Exist(addr gethCommon.Address) bool {
	exist, err := db.lastestView().Exist(addr)
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
	// TODO: If a state object with the address already exists the balance is carried over to the new account.
	// Carrying over the balance ensures that Ether doesn't disappear.
	db.lastestView().CreateAccount(addr)
}

// IsCreated returns true if address is recently created (context of a transaction)
func (db *StateDB) IsCreated(addr gethCommon.Address) bool {
	return db.lastestView().IsCreated(addr)
}

// Suicide flags the address for deletion.
//
// while this address exists for the rest of transaction,
// the balance of this account is return zero after the Suicide call.
func (db *StateDB) Suicide(addr gethCommon.Address) bool {
	success, err := db.lastestView().Suicide(addr)
	db.handleError(err)
	return success
}

// HasSuicided returns true if address is flaged with suicide.
func (db *StateDB) HasSuicided(addr gethCommon.Address) bool {
	return db.lastestView().HasSuicided(addr)
}

// SubBalance substitutes the amount from the balance of the given address
func (db *StateDB) SubBalance(addr gethCommon.Address, amount *big.Int) {
	err := db.lastestView().SubBalance(addr, amount)
	db.handleError(err)
}

// SubBalance adds the amount from the balance of the given address
func (db *StateDB) AddBalance(addr gethCommon.Address, amount *big.Int) {
	err := db.lastestView().AddBalance(addr, amount)
	db.handleError(err)
}

// GetBalance returns the balance of the given address
func (db *StateDB) GetBalance(addr gethCommon.Address) *big.Int {
	bal, err := db.lastestView().GetBalance(addr)
	db.handleError(err)
	return bal
}

// GetNonce returns the nonce of the given address
func (db *StateDB) GetNonce(addr gethCommon.Address) uint64 {
	nonce, err := db.lastestView().GetNonce(addr)
	db.handleError(err)
	return nonce
}

// SetNonce sets the nonce value for the given address
func (db *StateDB) SetNonce(addr gethCommon.Address, nonce uint64) {
	db.lastestView().SetNonce(addr, nonce)
}

func (db *StateDB) GetCodeHash(addr gethCommon.Address) gethCommon.Hash {
	hash, err := db.lastestView().GetCodeHash(addr)
	db.handleError(err)
	return hash
}

func (db *StateDB) GetCode(addr gethCommon.Address) []byte {
	code, err := db.lastestView().GetCode(addr)
	db.handleError(err)
	return code
}

func (db *StateDB) SetCode(addr gethCommon.Address, code []byte) {
	db.lastestView().SetCode(addr, code)
}

func (db *StateDB) GetCodeSize(addr gethCommon.Address) int {
	codeSize, err := db.lastestView().GetCodeSize(addr)
	db.handleError(err)
	return codeSize
}

func (db *StateDB) AddRefund(amount uint64) {
	db.lastestView().AddRefund(amount)
}

func (db *StateDB) SubRefund(amount uint64) {
	db.lastestView().SubRefund(amount)
}

func (db *StateDB) GetRefund() uint64 {
	return db.lastestView().GetRefund()
}

func (db *StateDB) GetCommittedState(addr gethCommon.Address, key gethCommon.Hash) gethCommon.Hash {
	value, err := db.baseView.GetState(types.SlotAddress{Address: addr, Key: key})
	db.handleError(err)
	return value
}

func (db *StateDB) GetState(addr gethCommon.Address, key gethCommon.Hash) gethCommon.Hash {
	state, err := db.lastestView().GetState(types.SlotAddress{Address: addr, Key: key})
	db.handleError(err)
	return state
}

func (db *StateDB) SetState(addr gethCommon.Address, key gethCommon.Hash, value gethCommon.Hash) {
	db.lastestView().SetState(types.SlotAddress{Address: addr, Key: key}, value)
}

func (db *StateDB) GetTransientState(addr gethCommon.Address, key gethCommon.Hash) gethCommon.Hash {
	return db.lastestView().GetTransientState(types.SlotAddress{Address: addr, Key: key})
}

func (db *StateDB) SetTransientState(addr gethCommon.Address, key, value gethCommon.Hash) {
	db.lastestView().SetTransientState(types.SlotAddress{Address: addr, Key: key}, value)
}

func (db *StateDB) AddressInAccessList(addr gethCommon.Address) bool {
	return db.lastestView().AddressInAccessList(addr)
}

func (db *StateDB) SlotInAccessList(addr gethCommon.Address, key gethCommon.Hash) (addressOk bool, slotOk bool) {
	return db.lastestView().SlotInAccessList(types.SlotAddress{Address: addr, Key: key})
}

// AddAddressToAccessList adds the given address to the access list.
func (db *StateDB) AddAddressToAccessList(addr gethCommon.Address) {
	db.lastestView().AddAddressToAccessList(addr)
}

// AddSlotToAccessList adds the given (address,slot) to the access list.
func (db *StateDB) AddSlotToAccessList(addr gethCommon.Address, key gethCommon.Hash) {
	db.lastestView().AddSlotToAccessList(types.SlotAddress{Address: addr, Key: key})
}

func (db *StateDB) AddLog(log *gethTypes.Log) {
	db.lastestView().AddLog(log)
}

func (db *StateDB) AddPreimage(hash gethCommon.Hash, data []byte) {
	db.lastestView().AddPreimage(hash, data)
}

func (db *StateDB) RevertToSnapshot(index int) {
	if index > len(db.views) {
		db.dbErr = fmt.Errorf("invalid revert")
		return
	}
	db.views = db.views[:index]
}

func (db *StateDB) Snapshot() int {
	newView := db.lastestView().NewChildView()
	db.views = append(db.views, newView)
	return len(db.views) - 1
}

func (db *StateDB) lastestView() *DeltaView {
	return db.views[len(db.views)-1]
}

func (db *StateDB) Logs(
	blockHash gethCommon.Hash,
	blockNumber uint64,
	txHash gethCommon.Hash,
	txIndex uint,
) []*gethTypes.Log {
	allLogs := make([]*gethTypes.Log, 0)
	for _, view := range db.views {
		for _, log := range view.Logs() {
			log.BlockNumber = blockNumber
			log.BlockHash = blockHash
			log.TxHash = txHash
			log.TxIndex = txIndex
			allLogs = append(allLogs, log)
		}
	}
	return allLogs
}

func (db *StateDB) Preimages() map[gethCommon.Hash][]byte {
	preImages := make(map[gethCommon.Hash][]byte, 0)
	for _, view := range db.views {
		for k, v := range view.Preimages() {
			preImages[k] = v
		}
	}
	return preImages
}

func (db *StateDB) Commit() error {
	// return error if any has been acumulated
	if db.dbErr != nil {
		return db.dbErr
	}

	var err error

	// iterate views and collect dirty addresses and slots
	addresses := make(map[gethCommon.Address]interface{})
	slots := make(map[types.SlotAddress]interface{})
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

	// update accounts
	for _, addr := range sortedAddresses {
		// TODO check if address is
		if db.HasSuicided(addr) {
			err = db.baseView.DeleteAccount(addr)
			if err != nil {
				return err
			}
			continue
		}
		if db.IsCreated(addr) {
			err = db.baseView.CreateAccount(
				addr,
				db.GetBalance(addr),
				db.GetNonce(addr),
				db.GetCode(addr),
				db.GetCodeHash(addr),
			)
			if err != nil {
				return err
			}
			continue
		}
		err = db.baseView.UpdateAccount(
			addr,
			db.GetBalance(addr),
			db.GetNonce(addr),
			db.GetCode(addr),
			db.GetCodeHash(addr),
		)
		if err != nil {
			return err
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
		err = db.baseView.UpdateSlot(
			sk,
			db.GetState(sk.Address, sk.Key),
		)
		if err != nil {
			return err
		}
	}

	// don't purge views yet, people might call the logs etc
	return db.baseView.Commit()
}

// Error returns the memorized database failure occurred earlier.
func (s *StateDB) Error() error {
	return s.dbErr
}

// handleError capture the first non-nil error it is called with.
func (s *StateDB) handleError(err error) {
	if err == nil {
		return
	}

	var atreeFatalError *atree.FatalError
	// if is a atree fatal error or fvm fatal error (the second one captures external errors)
	if stdErrors.As(err, &atreeFatalError) || errors.IsFailure(err) {
		panic(types.NewFatalError(err))
	}

	// already no error is captured
	if s.dbErr == nil {
		s.dbErr = types.NewDatabaseError(err)
	}
}

func (db *StateDB) Prepare(rules gethParams.Rules, sender, coinbase gethCommon.Address, dest *gethCommon.Address, precompiles []gethCommon.Address, txAccesses gethTypes.AccessList) {
	if rules.IsBerlin {
		// TODO: figure out this Clear out any leftover from previous executions
		// db.interim.ResetAccessList()

		// no need for mutation
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
	// TODO figure out these
	// Reset transient storage at the beginning of transaction execution
	// db.ResetTransientStorage()
}

func (db *StateDB) Reset() error {
	// TODO: implement me
	// TODO we might not need to recreate the base view and reuse it
	// bv := NewBaseView(db.ledger, db.root)
	// db.baseView = bv
	// db.views = []*DeltaView{NewDeltaView(bv)}
	// db.dbErr = nil
	return nil
}
