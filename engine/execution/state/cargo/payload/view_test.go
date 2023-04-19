package payload_test

import (
	"errors"
	"testing"

	"github.com/onflow/flow-go/engine/execution/state/cargo/payload"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	RegisterValueAtFunc    func(height uint64, blockID flow.Identifier, id flow.RegisterID) (value flow.RegisterValue, err error)
	CommitBlockFunc        func(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error
	LastCommittedBlockFunc func() (*flow.Header, error)
}

func (s *mockStorage) RegisterValueAt(height uint64, blockID flow.Identifier, id flow.RegisterID) (value flow.RegisterValue, err error) {
	return s.RegisterValueAtFunc(height, blockID, id)
}

func (s *mockStorage) CommitBlock(header *flow.Header, update map[flow.RegisterID]flow.RegisterValue) error {
	return s.CommitBlockFunc(header, update)
}

func (s *mockStorage) LastCommittedBlock() (*flow.Header, error) {
	return s.LastCommittedBlockFunc()
}

func TestOracleView(t *testing.T) {

	lastHeader := unittest.BlockHeaderFixture()
	data := map[flow.RegisterID]flow.RegisterValue{
		{Owner: "A", Key: "B"}: []byte("ABC"),
		{Owner: "C", Key: "D"}: []byte("CDE"),
	}

	storage := &mockStorage{
		RegisterValueAtFunc: func(h uint64, _ flow.Identifier, _ flow.RegisterID) (value flow.RegisterValue, err error) {
			if lastHeader.Height == h {
				return nil, nil
			}
			return nil, errors.New("wrong header was called")
		},
		CommitBlockFunc: func(header *flow.Header, inp map[flow.RegisterID]flow.RegisterValue) error {
			lastHeader = header
			require.Equal(t, data, inp)
			return nil
		},
		LastCommittedBlockFunc: func() (*flow.Header, error) {
			return lastHeader, nil
		},
	}

	ov := payload.NewOracleView(storage)

	_, err := ov.Get(lastHeader.Height, lastHeader.ID(), flow.RegisterID{})
	require.NoError(t, err)

	_, err = ov.Get(lastHeader.Height-1, flow.ZeroID, flow.RegisterID{})
	require.Error(t, err)

	_, err = ov.Get(lastHeader.Height+1, flow.ZeroID, flow.RegisterID{})
	require.Error(t, err)

	newHeader := unittest.BlockHeaderFixture()

	err = ov.MergeView(newHeader, payload.NewInFlightView(data, nil))
	require.NoError(t, err)

	_, err = ov.Get(newHeader.Height, newHeader.ID(), flow.RegisterID{})
	require.NoError(t, err)
}
