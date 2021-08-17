package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusSigPackerImpl struct {
	committees hotstuff.Committee
}

func NewConsensusSigPackerImpl(committees hotstuff.Committee) *ConsensusSigPackerImpl {
	return &ConsensusSigPackerImpl{
		committees: committees,
	}
}

func (p *ConsensusSigPackerImpl) Pack(sig *hotstuff.AggregatedSignatureData) ([]flow.Identifier, []byte, error) {
	panic("to be implemented")
}

func (p *ConsensusSigPackerImpl) Unpack(signerIDs []flow.Identifier, sigData []byte) (*hotstuff.AggregatedSignatureData, error) {
	panic("to be implemented")
}
