package types

type Vote struct {
	View      uint64
	BlockMRH  MRH
	Signature Signature
}
