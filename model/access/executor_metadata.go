package access

import (
	"github.com/onflow/flow-go/model/flow"
)

type ExecutorMetadata struct {
	ExecutionResultID flow.Identifier
	ExecutorIDs       flow.IdentifierList
}

func (m ExecutorMetadata) IsEmpty() bool {
	if m.ExecutionResultID == flow.ZeroID && len(m.ExecutorIDs) == 0 {
		return true
	}

	return false
}
