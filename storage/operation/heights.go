package operation

import "github.com/onflow/flow-go/storage"

func RetrieveFinalizedHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeFinalizedHeight), height)
}
