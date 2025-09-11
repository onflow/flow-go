package access

import (
	"github.com/onflow/flow-go/model/flow"
)

type ExecutorMetadata struct {
	ExecutionResultID flow.Identifier
	ExecutorIDs       flow.IdentifierList
}
