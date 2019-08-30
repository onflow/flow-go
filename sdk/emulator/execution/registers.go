package execution

// Registers is a map of register values.
type Registers map[string][]byte

func (r Registers) MergeWith(registers Registers) {
	for key, value := range registers {
		r[key] = value
	}
}

func (r Registers) NewView() *RegistersView {
	return &RegistersView{
		new: make(Registers),
		old: r,
	}
}

// RegistersView provides a read-only view into an existing registers state.
//
// Values are written to a temporary register cache that can later be
// committed to the world state.
type RegistersView struct {
	new Registers
	old Registers
}

func (r *RegistersView) UpdatedRegisters() Registers {
	return r.new
}

func (r *RegistersView) Get(key string) (value []byte, exists bool) {
	value, exists = r.new[key]
	if exists {
		return value, exists
	}

	value, exists = r.old[key]
	return value, exists
}

func (r *RegistersView) Set(key string, value []byte) {
	r.new[key] = value
}
