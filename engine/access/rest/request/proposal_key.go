package request

import "github.com/onflow/flow-go/model/flow"

type proposalKeyBody struct {
	Address        string `json:"address"`
	KeyIndex       string `json:"key_index"`
	SequenceNumber string `json:"sequence_number"`
}

type ProposalKey flow.ProposalKey

func (p *ProposalKey) Parse(raw proposalKeyBody) error {
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
