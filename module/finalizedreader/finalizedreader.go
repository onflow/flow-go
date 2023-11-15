package finalizedreader

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type FinalizedReader struct {
	headers storage.Headers
}

func NewFinalizedReader(headers storage.Headers) *FinalizedReader {
	return &FinalizedReader{
		headers: headers,
	}
}

func (r *FinalizedReader) FinalizedBlockIDAtHeight(height uint64) (flow.Identifier, error) {
	header, err := r.headers.ByHeight(height)
	if err != nil {
		return flow.ZeroID, err
	}

	return header.ID(), nil
}
