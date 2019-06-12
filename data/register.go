package data

type Registers map[string][]byte

func (r *Registers) Update(registers Registers) {
	for id, value := range registers {
		(*r)[id] = value
	}
}
