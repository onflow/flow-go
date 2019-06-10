package data

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks			map[Hash]Block
	Collections		map[Hash]Collection
	Transactions	map[Hash]Transaction
	Blockchain		[]Block
}
