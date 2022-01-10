package request

import (
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/model/flow"
)

type ProposalKey flow.ProposalKey

func (p *ProposalKey) Parse(raw models.ProposalKey) error {
	var address Address
	err := address.Parse(raw.Address)
	if err != nil {
		return err
	}

	keyIndex, err := toUint64(raw.KeyIndex)
	if err != nil {
		return err
	}

	seqNumber, err := toUint64(raw.SequenceNumber)
	if err != nil {
		return err
	}

	*p = ProposalKey(flow.ProposalKey{
		Address:        address.Flow(),
		KeyIndex:       keyIndex,
		SequenceNumber: seqNumber,
	})
	return nil
}

func (p ProposalKey) Flow() flow.ProposalKey {
	return flow.ProposalKey(p)
}
