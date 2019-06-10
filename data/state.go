package data

type WorldState struct {
	Blocks			map[Hash]Block
	Collections		map[Hash]Collection
	Transactions	map[Hash]Transaction
	Blockchain		[]Block
}