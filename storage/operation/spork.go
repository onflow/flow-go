package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func IndexSporkRootBlock(w storage.Writer, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeSporkRootBlockID), blockID)
}

// RetrieveSporkRootBlockID retrieves the spork root block ID for the present spork.
func RetrieveSporkRootBlockID(r storage.Reader, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeSporkRootBlockID), blockID)
}
