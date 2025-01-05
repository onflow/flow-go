package parser

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

type ProposalKey flow.ProposalKey

func (p *ProposalKey) Parse(raw interface{}) error {
	sigMap, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("'proposal_key' must be a JSON object")
	}

	addressStr, ok := sigMap["address"].(string)
	if !ok {
		return fmt.Errorf("'address' must be a string")
	}
	address, err := flow.StringToAddress(addressStr)
	if err != nil {
		return fmt.Errorf("invalid 'address': %w", err)
	}

	keyIndexStr, ok := sigMap["key_index"].(string)
	if !ok {
		return fmt.Errorf("'key_index' must be a string")
	}
	keyIndex, err := util.ToUint32(keyIndexStr)
	if err != nil {
		return fmt.Errorf("invalid 'key_index': %w", err)
	}

	sequenceNumberStr, ok := sigMap["sequence_number"].(string)
	if !ok {
		return fmt.Errorf("'sequence_number' must be a string")
	}
	sequenceNumber, err := util.ToUint64(sequenceNumberStr)
	if err != nil {
		return fmt.Errorf("invalid 'sequence_number': %w", err)
	}

	*p = ProposalKey{
		Address:        address,
		KeyIndex:       keyIndex,
		SequenceNumber: sequenceNumber,
	}

	return nil
}

func (p ProposalKey) Flow() flow.ProposalKey {
	return flow.ProposalKey(p)
}
