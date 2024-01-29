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

	// dirtyAddresses keeps a set of addresses with changes
	dirtyAddresses map[gethCommon.Address]struct{}
	// created keeps a set of recently created addresses
	created map[gethCommon.Address]struct{}
	// toBeDestructed keeps a set of addresses flagged to be destructed at the
	// end of transaction, it also keeps the balance of the addresses before destruction
	toBeDestructed map[gethCommon.Address]*big.Int
	// is a flag used to track accounts that has been flagged for
	// destruction but recreated later
	recreated map[gethCommon.Address]struct{}
	// balances keeps the changes to the account balances
	balances map[gethCommon.Address]*big.Int
	// nonces keeps the changes to the account nonces
	nonces map[gethCommon.Address]uint64
	// codes keeps the changes to the account codes
	codes map[gethCommon.Address][]byte
	// codeHashes keeps the changes to account code hashes
	codeHashes map[gethCommon.Address]gethCommon.Hash

	// slots keeps a set of slots that has been changed in this view
	slots map[types.SlotAddress]gethCommon.Hash

	// transient storage
	transient map[types.SlotAddress]gethCommon.Hash

	// access lists
	accessListAddresses map[gethCommon.Address]struct{}
	accessListSlots     map[types.SlotAddress]struct{}

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

		dirtyAddresses: make(map[gethCommon.Address]struct{}),
		created:        make(map[gethCommon.Address]struct{}),
		toBeDestructed: make(map[gethCommon.Address]*big.Int),
		recreated:      make(map[gethCommon.Address]struct{}),
		balances:       make(map[gethCommon.Address]*big.Int),
		nonces:         make(map[gethCommon.Address]uint64),
		codes:          make(map[gethCommon.Address][]byte),
		codeHashes:     make(map[gethCommon.Address]gethCommon.Hash),

		slots: make(map[types.SlotAddress]gethCommon.Hash),

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
	_, found = d.toBeDestructed[addr]
	if found {
		return true, nil
	}
	return d.parent.Exist(addr)
}

// CreateAccount creates a new account for the given address
//
// if address already extists (even if destructed), carry over the balance
// and reset the data from the orginal account.
func (d *DeltaView) CreateAccount(addr gethCommon.Address) error {
	// if is already created return
	if d.IsCreated(addr) {
		return nil
	}
	exist, err := d.Exist(addr)
	if err != nil {
		return err
	}
	if exist {
		// check if already destructed
		destructed, balance := d.HasSelfDestructed(addr)
		if !destructed {
			balance, err = d.GetBalance(addr)
			if err != nil {
				return err
			}
			err = d.SelfDestruct(addr)
			if err != nil {
				return err
			}
		}

		d.nonces[addr] = 0
		d.codes[addr] = nil
		d.codeHashes[addr] = gethTypes.EmptyCodeHash
		// carrying over the balance. (legacy behaviour of the Geth stateDB)
		d.balances[addr] = balance

		// flag addr as recreated, this flag helps with postponing deletion of slabs
		// otherwise we have to iterate over all slabs of this account and set the to nil
		d.recreated[addr] = struct{}{}

		// remove slabs from cache related to this account
		for k := range d.slots {
			if k.Address == addr {
				delete(d.slots, k)
			}
		}
	}
	d.dirtyAddresses[addr] = struct{}{}
	d.created[addr] = struct{}{}
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

// HasSelfDestructed returns true if address has been flagged for destruction
// it also returns the balance of the address before the destruction call
func (d *DeltaView) HasSelfDestructed(addr gethCommon.Address) (bool, *big.Int) {
	bal, found := d.toBeDestructed[addr]
	if found {
		return true, bal
	}
	return d.parent.HasSelfDestructed(addr)
}

// SelfDestruct sets a flag to destruct the account at the end of transaction
//
// if an account has been created in this transaction, it would return an error
func (d *DeltaView) SelfDestruct(addr gethCommon.Address) error {
	// if it has been recently created, calling self destruct is not a valid operation
	if d.IsCreated(addr) {
		return fmt.Errorf("invalid operation, can't selfdestruct an account that is just created")
	}

	// if it doesn't exist, return false
	exists, err := d.Exist(addr)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	// flag the account for destruction and capture the balance
	// before destruction
	d.toBeDestructed[addr], err = d.GetBalance(addr)
	if err != nil {
		return err
	}
	// flag the address as dirty
	d.dirtyAddresses[addr] = struct{}{}

	// set balance to zero
	d.balances[addr] = new(big.Int)
	return nil
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
	val, found := d.slots[sk]
	if found {
		return val, nil
	}
	// if address is deleted in the scope of this delta view,
	// don't go backward. this has been done to skip the step to iterate
	// over all the state slabs and delete them.
	_, recreated := d.recreated[sk.Address]
	if recreated {
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
	// if the value hasn't changed, skip
	if value == lastValue {
		return nil
	}
	d.slots[sk] = value
	return nil
}

// GetTransientState returns the value of the slot of the transient state
func (d *DeltaView) GetTransientState(sk types.SlotAddress) gethCommon.Hash {
	if d.transient != nil {
		val, found := d.transient[sk]
		if found {
			return val
		}
	}
	return d.parent.GetTransientState(sk)
}

// SetTransientState adds sets a value for the given slot of the transient storage
func (d *DeltaView) SetTransientState(sk types.SlotAddress, value gethCommon.Hash) {
	if d.transient == nil {
		d.transient = make(map[types.SlotAddress]gethCommon.Hash)
	}
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
	if d.accessListAddresses != nil {
		_, addressFound := d.accessListAddresses[addr]
		if addressFound {
			return true
		}
	}
	return d.parent.AddressInAccessList(addr)
}

// AddAddressToAccessList adds an address to the access list
func (d *DeltaView) AddAddressToAccessList(addr gethCommon.Address) bool {
	if d.accessListAddresses == nil {
		d.accessListAddresses = make(map[gethCommon.Address]struct{})
	}

	addrPresent := d.AddressInAccessList(addr)
	d.accessListAddresses[addr] = struct{}{}
	return !addrPresent
}

// SlotInAccessList checks if the slot is in the access list
func (d *DeltaView) SlotInAccessList(sk types.SlotAddress) (addressOk bool, slotOk bool) {
	addressFound := d.AddressInAccessList(sk.Address)
	if d.accessListSlots != nil {
		_, slotFound := d.accessListSlots[sk]
		if slotFound {
			return addressFound, true
		}
	}
	_, slotFound := d.parent.SlotInAccessList(sk)
	return addressFound, slotFound
}

// AddSlotToAccessList adds a slot to the access list
// it also adds the address to the address list
func (d *DeltaView) AddSlotToAccessList(sk types.SlotAddress) (addrAdded bool, slotAdded bool) {
	addrPresent, slotPresent := d.SlotInAccessList(sk)
	if d.accessListAddresses == nil {
		d.accessListAddresses = make(map[gethCommon.Address]struct{})
	}
	d.accessListAddresses[sk.Address] = struct{}{}
	if d.accessListSlots == nil {
		d.accessListSlots = make(map[types.SlotAddress]struct{})
	}
	d.accessListSlots[sk] = struct{}{}
	return !addrPresent, !slotPresent
}

// AddLog appends a log to the log collection
func (d *DeltaView) AddLog(log *gethTypes.Log) {
	if d.logs == nil {
		d.logs = make([]*gethTypes.Log, 0)
	}
	d.logs = append(d.logs, log)
}

// Logs returns the logs that has been captured in this view
func (d *DeltaView) Logs() []*gethTypes.Log {
	return d.logs
}

// AddPreimage adds a preimage
func (d *DeltaView) AddPreimage(hash gethCommon.Hash, preimage []byte) {
	if d.preimages == nil {
		d.preimages = make(map[gethCommon.Hash][]byte)
	}

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
func (d *DeltaView) DirtyAddresses() map[gethCommon.Address]struct{} {
	return d.dirtyAddresses
}

// DirtySlots returns a set of slots that has been updated in this view
func (d *DeltaView) DirtySlots() map[types.SlotAddress]struct{} {
	dirtySlots := make(map[types.SlotAddress]struct{})
	for sk := range d.slots {
		dirtySlots[sk] = struct{}{}
	}
	return dirtySlots
}
