// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

type Seal struct {
	BlockID           Identifier
	ExecutionResultID Identifier
	PreviousState     StateCommitment
	FinalState        StateCommitment
	Signature         crypto.Signature
}

func (s Seal) Body() interface{} {
	return struct {
		BlockID       Identifier
		PreviousState StateCommitment
		FinalState    StateCommitment
	}{
		BlockID:       s.BlockID,
		PreviousState: s.PreviousState,
		FinalState:    s.FinalState,
	}
}

func (s Seal) ID() Identifier {
	return MakeID(s.Body())
}

func (s Seal) Checksum() Identifier {
	return MakeID(s)
}
