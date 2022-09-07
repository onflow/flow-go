package cmd

import (
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
)

type Reader struct {
	persister *persister.Persister
}

func NewReader(persister *persister.Persister) *Reader {
	return &Reader{
		persister: persister,
	}
}

// TODO replace with GetLivenessData, GetSafetyData https://github.com/dapperlabs/flow-go/issues/6388
func (r *Reader) GetHotstuffView() (uint64, error) {
	panic("not implemented - to be removed")
}
