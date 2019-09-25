package types

import "github.com/dapperlabs/flow-go/pkg/crypto"

// Registers is a map of register values.
type Registers map[string][]byte

// MergeWith inserts all key/value pairs from another register set into this one.
func (r Registers) MergeWith(registers Registers) {
	for key, value := range registers {
		r[key] = value
	}
}

// NewView returns a new read-only view onto this register set.
func (r Registers) NewView() *RegistersView {
	return &RegistersView{
		new: make(Registers),
		old: r,
	}
}

// RegistersView provides a read-only view into an existing register set.
//
// Values are written to a temporary register cache that can later be
// committed to the world state.
type RegistersView struct {
	new Registers
	old Registers
}

// UpdatedRegisters returns the set of registers that were written to this view.
func (r *RegistersView) UpdatedRegisters() Registers {
	return r.new
}

// Get gets a register from this view.
func (r *RegistersView) Get(key string) (value []byte, exists bool) {
	value, exists = r.new[key]
	if exists {
		return value, exists
	}

	value, exists = r.old[key]
	return value, exists
}

// Set sets a register in this view.
func (r *RegistersView) Set(key string, value []byte) {
	r.new[key] = value
}

type IntermediateRegisters struct {
	TransactionHash crypto.Hash
	Registers       Registers
	ComputeUsed     uint64
}
