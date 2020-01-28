// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

type Seal struct {
	BlockID      Identifier
	ParentCommit StateCommitment
	StateCommit  StateCommitment
	Signature    crypto.Signature
}

func (s Seal) Body() interface{} {
	return struct {
		BlockID      Identifier
		ParentCommit StateCommitment
		StateCommit  StateCommitment
	}{
		BlockID:      s.BlockID,
		ParentCommit: s.ParentCommit,
		StateCommit:  s.StateCommit,
	}
}

func (s Seal) ID() Identifier {
	return MakeID(s.Body())
}

func (s Seal) Checksum() Identifier {
	return MakeID(s)
}
