package data

// TODO: define format for register keys and values

// Registers is a map of register values.
type Registers map[string][]byte

// Update merges this register map with the values of another register map.
//
// If the other map contains keys that already exist in this map, the values
// in this map are overwritten.
func (r *Registers) Update(registers Registers) {
	for id, value := range registers {
		(*r)[id] = value
	}
}
