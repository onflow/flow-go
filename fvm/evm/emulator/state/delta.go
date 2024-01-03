package state

import (
	"fmt"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// DeltaView captures the changes to the state during the execution
//
// for most of the read calls it checks its change logs and if no record is
// found it would redirect the call to the parent view.
type DeltaView struct {
	parent types.ReadOnlyView

	// account changes
	dirtyAddresses map[gethCommon.Address]interface{}
	created        map[gethCommon.Address]interface{}
	suicided       map[gethCommon.Address]interface{}
	deleted        map[gethCommon.Address]interface{}
	balances       map[gethCommon.Address]*big.Int
	nonces         map[gethCommon.Address]uint64
	codes          map[gethCommon.Address][]byte
	codeHashes     map[gethCommon.Address]gethCommon.Hash

	// states changes
	dirtySlots map[types.SlotAddress]interface{}
	states     map[types.SlotAddress]gethCommon.Hash

	// transient storage
	transient map[types.SlotAddress]gethCommon.Hash

	// access lists
	accessListAddresses map[gethCommon.Address]interface{}
	accessListSlots     map[types.SlotAddress]interface{}

	// logs
	logs []*gethTypes.Log

	// preimages
	preimages map[gethCommon.Hash][]byte

	// refund
	refund uint64
}

var _ types.HotView = &DeltaView{}

// NewDeltaView constructs a new delta view
func NewDeltaView(parent types.ReadOnlyView) *DeltaView {
	return &DeltaView{
		parent: parent,

		dirtyAddresses:      make(map[gethCommon.Address]interface{}),
		created:             make(map[gethCommon.Address]interface{}),
		suicided:            make(map[gethCommon.Address]interface{}),
		deleted:             make(map[gethCommon.Address]interface{}),
		balances:            make(map[gethCommon.Address]*big.Int),
		nonces:              make(map[gethCommon.Address]uint64),
		codes:               make(map[gethCommon.Address][]byte),
		codeHashes:          make(map[gethCommon.Address]gethCommon.Hash),
		dirtySlots:          make(map[types.SlotAddress]interface{}),
		states:              make(map[types.SlotAddress]gethCommon.Hash),
		transient:           make(map[types.SlotAddress]gethCommon.Hash),
		accessListAddresses: make(map[gethCommon.Address]interface{}),
		accessListSlots:     make(map[types.SlotAddress]interface{}),
		logs:                make([]*gethTypes.Log, 0),
		preimages:           make(map[gethCommon.Hash][]byte),

		// for refund we just copy the data
		refund: parent.GetRefund(),
	}
}

// NewChildView constructs a new delta view having the current view as parent
func (d *DeltaView) NewChildView() *DeltaView {
	return NewDeltaView(d)
}

// Exist returns true if address exists
//
// it also returns true for both newly created accounts or accounts that has been flagged for deletion
func (d *DeltaView) Exist(addr gethCommon.Address) (bool, error) {
	_, found := d.created[addr]
	if found {
		return true, nil
	}
	_, found = d.suicided[addr]
	if found {
		return true, nil
	}
	return d.parent.Exist(addr)
}

// CreateAccount creates a new account for the given address
func (d *DeltaView) CreateAccount(addr gethCommon.Address) error {
	// If a address already exists the balance is carried over to the new account.
	// Carrying over the balance ensures that Ether doesn't disappear. (legacy behaviour of the Geth stateDB)
	exist, err := d.Exist(addr)
	if err != nil {
		return err
	}
	var bal *big.Int
	if exist {
		bal, err = d.GetBalance(addr)
		if err != nil {
			return err
		}
	}

	d.created[addr] = struct{}{}
	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}

	if exist {
		d.AddBalance(addr, bal)
	}

	// if has already suicided
	if d.HasSuicided(addr) {
		// balance has already been set to zero
		d.nonces[addr] = 0
		d.codes[addr] = nil
		d.codeHashes[addr] = gethTypes.EmptyCodeHash

		// flag addr as deleted, this flag helps with postponing deletion of slabs
		// otherwise we have to iterate over all slabs of this account and set the to nil
		d.deleted[addr] = struct{}{}

		// remove slabs from cache related to this account
		for k := range d.states {
			if k.Address == addr {
				delete(d.states, k)
			}
		}
	}
	return nil
}

// IsCreated returns true if address has been created in this tx
func (d *DeltaView) IsCreated(addr gethCommon.Address) bool {
	_, found := d.created[addr]
	if found {
		return true
	}
	return d.parent.IsCreated(addr)
}

// HasSuicided returns true if address has been flagged for deletion
func (d *DeltaView) HasSuicided(addr gethCommon.Address) bool {
	_, found := d.suicided[addr]
	if found {
		return true
	}
	return d.parent.HasSuicided(addr)
}

// Suicide sets a flag to delete the account at the end of transaction
func (d *DeltaView) Suicide(addr gethCommon.Address) (bool, error) {
	// if it doesn't exist, return false
	exists, err := d.Exist(addr)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	// flag the account for deletion
	d.suicided[addr] = struct{}{}

	// set balance to zero
	d.balances[addr] = new(big.Int)

	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}
	return true, nil
}

// GetBalance returns the balance of the given address
func (d *DeltaView) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	val, found := d.balances[addr]
	if found {
		return val, nil
	}
	// if newly created and no balance is set yet
	_, newlyCreated := d.created[addr]
	if newlyCreated {
		return big.NewInt(0), nil
	}
	return d.parent.GetBalance(addr)
}

// AddBalance adds the amount to the current balance of the given address
func (d *DeltaView) AddBalance(addr gethCommon.Address, amount *big.Int) error {
	// if amount is 0 skip
	if amount.Sign() == 0 {
		return nil
	}
	// get the latest balance
	orgBalance, err := d.GetBalance(addr)
	if err != nil {
		return err
	}
	// update the balance
	newBalance := new(big.Int).Add(orgBalance, amount)
	d.balances[addr] = newBalance

	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

// SubBalance subtracts the amount from the current balance of the given address
func (d *DeltaView) SubBalance(addr gethCommon.Address, amount *big.Int) error {
	// if amount is 0 skip
	if amount.Sign() == 0 {
		return nil
	}

	// get the latest balance
	orgBalance, err := d.GetBalance(addr)
	if err != nil {
		return err
	}

	// update the new balance
	newBalance := new(big.Int).Sub(orgBalance, amount)

	// if new balance is negative error
	if newBalance.Sign() < 0 {
		return fmt.Errorf("account balance is negative %d", newBalance)
	}

	// update the balance
	d.balances[addr] = newBalance

	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

// GetNonce returns the nonce of the given address
func (d *DeltaView) GetNonce(addr gethCommon.Address) (uint64, error) {
	val, found := d.nonces[addr]
	if found {
		return val, nil
	}
	// if newly created
	_, newlyCreated := d.created[addr]
	if newlyCreated {
		return 0, nil
	}
	return d.parent.GetNonce(addr)
}

// SetNonce sets the nonce for the given address
func (d *DeltaView) SetNonce(addr gethCommon.Address, nonce uint64) error {
	// update the nonce
	d.nonces[addr] = nonce

	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

// GetCode returns the code of the given address
func (d *DeltaView) GetCode(addr gethCommon.Address) ([]byte, error) {
	code, found := d.codes[addr]
	if found {
		return code, nil
	}
	// if newly created
	_, newlyCreated := d.created[addr]
	if newlyCreated {
		return nil, nil
	}
	return d.parent.GetCode(addr)
}

// GetCodeSize returns the code size of the given address
func (d *DeltaView) GetCodeSize(addr gethCommon.Address) (int, error) {
	code, err := d.GetCode(addr)
	return len(code), err
}

// GetCodeHash returns the code hash of the given address
func (d *DeltaView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	codeHash, found := d.codeHashes[addr]
	if found {
		return codeHash, nil
	}
	// if newly created
	_, newlyCreated := d.created[addr]
	if newlyCreated {
		return gethTypes.EmptyCodeHash, nil
	}
	return d.parent.GetCodeHash(addr)
}

// SetCode sets the code for the given address
func (d *DeltaView) SetCode(addr gethCommon.Address, code []byte) error {
	// update code
	d.codes[addr] = code

	// update code hash
	codeHash := gethTypes.EmptyCodeHash
	if len(code) > 0 {
		codeHash = gethCrypto.Keccak256Hash(code)
	}
	d.codeHashes[addr] = codeHash

	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

// GetState returns the value of the slot of the main state
func (d *DeltaView) GetState(sk types.SlotAddress) (gethCommon.Hash, error) {
	val, found := d.states[sk]
	if found {
		return val, nil
	}
	// if address is deleted in the scope of this delta view,
	// don't go backward. this has been done to skip the step to iterate
	// over all the state slabs and delete them.
	_, deleted := d.deleted[sk.Address]
	if deleted {
		return gethCommon.Hash{}, nil
	}
	return d.parent.GetState(sk)
}

// SetState adds sets a value for the given slot of the main storage
func (d *DeltaView) SetState(sk types.SlotAddress, value gethCommon.Hash) error {
	lastValue, err := d.GetState(sk)
	if err != nil {
		return err
	}
	// we skip the value is the same
	// this step might look not helping with performance but we kept it to
	// act similar to the Geth StateDB behaviour
	if value == lastValue {
		return nil
	}
	d.states[sk] = value
	d.dirtySlots[sk] = struct{}{}
	return nil
}

// GetTransientState returns the value of the slot of the transient state
func (d *DeltaView) GetTransientState(sk types.SlotAddress) gethCommon.Hash {
	val, found := d.transient[sk]
	if found {
		return val
	}
	return d.parent.GetTransientState(sk)
}

// SetTransientState adds sets a value for the given slot of the transient storage
func (d *DeltaView) SetTransientState(sk types.SlotAddress, value gethCommon.Hash) {
	d.transient[sk] = value
}

// GetRefund returns the total (gas) refund
func (d *DeltaView) GetRefund() uint64 {
	return d.refund
}

// AddRefund adds the amount to the total (gas) refund
func (d *DeltaView) AddRefund(amount uint64) error {
	d.refund += amount
	return nil
}

// SubRefund subtracts the amount from the total (gas) refund
func (d *DeltaView) SubRefund(amount uint64) error {
	if amount > d.refund {
		return fmt.Errorf("refund counter below zero (gas: %d > refund: %d)", amount, d.refund)
	}
	d.refund -= amount
	return nil
}

// AddressInAccessList checks if the address is in the access list
func (d *DeltaView) AddressInAccessList(addr gethCommon.Address) bool {
	_, addressFound := d.accessListAddresses[addr]
	if !addressFound {
		addressFound = d.parent.AddressInAccessList(addr)
	}
	return addressFound
}

// AddAddressToAccessList adds an address to the access list
func (d *DeltaView) AddAddressToAccessList(addr gethCommon.Address) bool {
	addrPresent := d.AddressInAccessList(addr)
	d.accessListAddresses[addr] = struct{}{}
	return !addrPresent
}

// SlotInAccessList checks if the slot is in the access list
func (d *DeltaView) SlotInAccessList(sk types.SlotAddress) (addressOk bool, slotOk bool) {
	addressFound := d.AddressInAccessList(sk.Address)
	_, slotFound := d.accessListSlots[sk]
	if !slotFound {
		_, slotFound = d.parent.SlotInAccessList(sk)
	}
	return addressFound, slotFound
}

// AddSlotToAccessList adds a slot to the access list
// it also adds the address to the address list
func (d *DeltaView) AddSlotToAccessList(sk types.SlotAddress) (addrAdded bool, slotAdded bool) {
	addrPresent, slotPresent := d.SlotInAccessList(sk)
	d.accessListAddresses[sk.Address] = struct{}{}
	d.accessListSlots[sk] = struct{}{}
	return !addrPresent, !slotPresent
}

// AddLog appends a log to the log collection
func (d *DeltaView) AddLog(log *gethTypes.Log) {
	d.logs = append(d.logs, log)
}

// Logs returns the logs that has been captured in this view
func (d *DeltaView) Logs() []*gethTypes.Log {
	return d.logs
}

// AddPreimage adds a preimage
func (d *DeltaView) AddPreimage(hash gethCommon.Hash, preimage []byte) {
	// make a copy (legacy behaviour)
	pi := make([]byte, len(preimage))
	copy(pi, preimage)
	d.preimages[hash] = pi
}

// Preimages returns a map of preimages
func (d *DeltaView) Preimages() map[gethCommon.Hash][]byte {
	return d.preimages
}

// DirtyAddresses returns a set of addresses that has been updated in this view
func (d *DeltaView) DirtyAddresses() map[gethCommon.Address]interface{} {
	return d.dirtyAddresses
}

// DirtySlots returns a set of slots that has been updated in this view
func (d *DeltaView) DirtySlots() map[types.SlotAddress]interface{} {
	return d.dirtySlots
}
