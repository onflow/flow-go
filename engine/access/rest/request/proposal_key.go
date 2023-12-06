package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

type ProposalKey flow.ProposalKey

func (p *ProposalKey) Parse(raw models.ProposalKey, chain flow.Chain) error {
	address, err := ParseAddress(raw.Address, chain)
	if err != nil {
		return err
	}

	keyIndex, err := util.ToUint64(raw.KeyIndex)
	if err != nil {
		return fmt.Errorf("invalid key index: %w", err)
	}

	seqNumber, err := util.ToUint64(raw.SequenceNumber)
	if err != nil {
		return fmt.Errorf("invalid sequence number: %w", err)
	}

	*p = ProposalKey(flow.ProposalKey{
		Address:        address,
		KeyIndex:       keyIndex,
		SequenceNumber: seqNumber,
	})
	return nil
}

func (p ProposalKey) Flow() flow.ProposalKey {
	return flow.ProposalKey(p)
}
