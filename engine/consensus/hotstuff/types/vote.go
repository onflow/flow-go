package types

type Vote struct {
	View      uint64
	BlockMRH  MRH
	Signature *Signature
}

func NewVote(view uint64, blockMRH MRH) *Vote {
	return &Vote{
		View:     view,
		BlockMRH: blockMRH,
	}
}
