package execution_indexer

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/storage/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func newIndexer() (*Indexer, *mock.Registers, *mock.Headers) {
	registerMock := &mock.Registers{}
	headerMock := &mock.Headers{}

	return &Indexer{
		registers:   registerMock,
		headers:     headerMock,
		last:        0,
		commitments: nil,
	}, registerMock, headerMock
}

func TestIndexer_StorePayloads(t *testing.T) {
	indexer, registerMock, _ := newIndexer()

	payloads := []*ledger.Payload{
		ledger.NewPayload(
			ledger.Key{KeyParts: []ledger.KeyPart{{1, []byte{0x01}}}},
			[]byte{0x02, 0x03},
		),
	}

	err := indexer.StorePayloads(payloads, 1)
	require.NoError(t, err)
}
