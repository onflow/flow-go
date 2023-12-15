package state

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/onflow/flow-go/fvm/evm/types"
)

type DeltaView struct {
	parent types.ReadOnlyView

	// account changes
	dirtyAddresses map[gethCommon.Address]interface{}
	created        map[gethCommon.Address]interface{}
	suicided       map[gethCommon.Address]interface{}
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

func NewDeltaView(parent types.ReadOnlyView) *DeltaView {
	return &DeltaView{
		parent: parent,

		dirtyAddresses:      make(map[gethCommon.Address]interface{}, 0),
		created:             make(map[gethCommon.Address]interface{}, 0),
		suicided:            make(map[gethCommon.Address]interface{}, 0),
		balances:            make(map[gethCommon.Address]*big.Int, 0),
		nonces:              make(map[gethCommon.Address]uint64, 0),
		codes:               make(map[gethCommon.Address][]byte, 0),
		codeHashes:          make(map[gethCommon.Address]gethCommon.Hash, 0),
		dirtySlots:          make(map[types.SlotAddress]interface{}, 0),
		states:              make(map[types.SlotAddress]gethCommon.Hash, 0),
		transient:           make(map[types.SlotAddress]gethCommon.Hash, 0),
		accessListAddresses: make(map[gethCommon.Address]interface{}, 0),
		accessListSlots:     make(map[types.SlotAddress]interface{}, 0),
		logs:                make([]*gethTypes.Log, 0),
		preimages:           make(map[gethCommon.Hash][]byte, 0),

		// for refund we just copy the data
		refund: parent.GetRefund(),
	}
}

func (d *DeltaView) NewChildView() *DeltaView {
	return NewDeltaView(d)
}

func (d *DeltaView) DirtyAddresses() map[gethCommon.Address]interface{} {
	return d.dirtyAddresses
}

func (d *DeltaView) DirtySlots() map[types.SlotAddress]interface{} {
	return d.dirtySlots
}

func (d *DeltaView) Logs() []*gethTypes.Log {
	return d.logs
}

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

func (d *DeltaView) HasSuicided(addr gethCommon.Address) bool {
	_, found := d.suicided[addr]
	if found {
		return true
	}
	return d.parent.HasSuicided(addr)
}

func (d *DeltaView) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	val, found := d.balances[addr]
	if found {
		return val, nil
	}
	return d.parent.GetBalance(addr)
}

func (d *DeltaView) GetNonce(addr gethCommon.Address) (uint64, error) {
	val, found := d.nonces[addr]
	if found {
		return val, nil
	}
	return d.parent.GetNonce(addr)
}

func (d *DeltaView) GetCode(addr gethCommon.Address) ([]byte, error) {
	code, found := d.codes[addr]
	if found {
		return code, nil
	}
	return d.parent.GetCode(addr)
}

func (d *DeltaView) GetCodeSize(addr gethCommon.Address) (int, error) {
	code, found := d.codes[addr]
	if found {
		return len(code), nil
	}
	return d.parent.GetCodeSize(addr)
}

func (d *DeltaView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	codeHash, found := d.codeHashes[addr]
	if found {
		return codeHash, nil
	}
	return d.parent.GetCodeHash(addr)
}

func (d *DeltaView) GetState(sk types.SlotAddress) (gethCommon.Hash, error) {
	val, found := d.states[sk]
	if found {
		return val, nil
	}
	return d.parent.GetState(sk)
}

func (d *DeltaView) GetTransientState(sk types.SlotAddress) gethCommon.Hash {
	val, found := d.transient[sk]
	if found {
		return val
	}
	return d.parent.GetTransientState(sk)
}

func (d *DeltaView) GetRefund() uint64 {
	return d.refund
}

func (d *DeltaView) AddressInAccessList(addr gethCommon.Address) bool {
	_, addressFound := d.accessListAddresses[addr]
	if !addressFound {
		addressFound = d.parent.AddressInAccessList(addr)
	}
	return addressFound
}

func (d *DeltaView) SlotInAccessList(sk types.SlotAddress) (addressOk bool, slotOk bool) {
	addressFound := d.AddressInAccessList(sk.Address)
	_, slotFound := d.accessListSlots[sk]
	if !slotFound {
		_, slotFound = d.parent.SlotInAccessList(sk)
	}
	return addressFound, slotFound
}

func (d *DeltaView) CreateAccount(addr gethCommon.Address) error {
	d.created[addr] = struct{}{}
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

func (d *DeltaView) Suicide(addr gethCommon.Address) (bool, error) {
	exists, err := d.Exist(addr)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	d.suicided[addr] = struct{}{}
	d.dirtyAddresses[addr] = struct{}{}
	return true, nil
}

func (d *DeltaView) AddBalance(addr gethCommon.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	orgBalance, err := d.GetBalance(addr)
	if err != nil {
		return err
	}
	newBalance := new(big.Int).Add(orgBalance, amount)
	d.balances[addr] = newBalance
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

func (d *DeltaView) SubBalance(addr gethCommon.Address, amount *big.Int) error {
	if amount.Sign() == 0 {
		return nil
	}
	orgBalance, err := d.GetBalance(addr)
	if err != nil {
		return err
	}
	newBalance := new(big.Int).Sub(orgBalance, amount)
	d.balances[addr] = newBalance
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

func (d *DeltaView) SetNonce(addr gethCommon.Address, nonce uint64) error {
	d.nonces[addr] = nonce
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

func (d *DeltaView) SetCode(addr gethCommon.Address, code []byte) error {
	codeHash := gethTypes.EmptyCodeHash
	if len(code) > 0 {
		codeHash = gethCrypto.Keccak256Hash(code)
	}
	d.codes[addr] = code
	d.codeHashes[addr] = codeHash
	d.dirtyAddresses[addr] = struct{}{}
	return nil
}

func (d *DeltaView) AddRefund(amount uint64) {
	d.refund += amount
}

func (d *DeltaView) SubRefund(amount uint64) {
	d.refund -= amount
}

func (d *DeltaView) SetTransientState(sk types.SlotAddress, value gethCommon.Hash) {
	d.transient[sk] = value
}

func (d *DeltaView) SetState(sk types.SlotAddress, value gethCommon.Hash) error {
	d.states[sk] = value
	return nil
}

func (d *DeltaView) AddAddressToAccessList(addr gethCommon.Address) {
	d.accessListAddresses[addr] = struct{}{}
}

func (d *DeltaView) AddSlotToAccessList(sk types.SlotAddress) {
	d.accessListSlots[sk] = struct{}{}
}

func (d *DeltaView) AddLog(log *gethTypes.Log) {
	d.logs = append(d.logs, log)
}

func (d *DeltaView) AddPreimage(hash gethCommon.Hash, input []byte) {
	d.preimages[hash] = input
}

// Commit for deltaview is a no-op
func (d *DeltaView) Commit() error {
	return nil
}
