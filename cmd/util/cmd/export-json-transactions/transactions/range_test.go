package transactions

import (
	"fmt"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFindBlockTransactions(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		// prepare two blocks
		// block 1 has 2 collections
		// block 2 has 1 collection
		col1 := unittest.ClusterPayloadFixture(1)
		col2 := unittest.ClusterPayloadFixture(2)
		col3 := unittest.ClusterPayloadFixture(3)

		b1 := unittest.BlockFixture(
			unittest.Block.WithHeight(4),
			unittest.Block.WithPayload(
				flow.Payload{
					Guarantees: []*flow.CollectionGuarantee{
						&flow.CollectionGuarantee{
							CollectionID:     col1.Collection.ID(),
							ReferenceBlockID: col1.ReferenceBlockID,
						},
						&flow.CollectionGuarantee{
							CollectionID:     col2.Collection.ID(),
							ReferenceBlockID: col2.ReferenceBlockID,
						},
					},
					ProtocolStateID: unittest.IdentifierFixture(),
				},
			),
		)

		b2 := unittest.BlockFixture(
			unittest.Block.WithHeight(5),
			unittest.Block.WithPayload(
				flow.Payload{
					Guarantees: []*flow.CollectionGuarantee{
						&flow.CollectionGuarantee{
							CollectionID:     col3.Collection.ID(),
							ReferenceBlockID: col3.ReferenceBlockID,
						},
					},
					ProtocolStateID: unittest.IdentifierFixture(),
				},
			),
		)

		// prepare dependencies
		storages, err := common.InitStorages(db, flow.Emulator)
		require.NoError(t, err)
		payloads, collections := storages.Payloads, storages.Collections
		snap4 := &mock.Snapshot{}
		snap4.On("Head").Return(b1.ToHeader(), nil)
		snap5 := &mock.Snapshot{}
		snap5.On("Head").Return(b2.ToHeader(), nil)
		state := &mock.State{}
		state.On("AtHeight", uint64(4)).Return(snap4, nil)
		state.On("AtHeight", uint64(5)).Return(snap5, nil)

		// store into database
		p1 := unittest.ProposalFromBlock(b1)
		p2 := unittest.ProposalFromBlock(b2)
		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				err := storages.Blocks.BatchStore(lctx, rw, p1)
				if err != nil {
					return err
				}

				return storages.Blocks.BatchStore(lctx, rw, p2)
			})
		})
		require.NoError(t, err)

		_, err = collections.Store(&col1.Collection)
		require.NoError(t, err)
		_, err = collections.Store(&col2.Collection)
		require.NoError(t, err)
		_, err = collections.Store(&col3.Collection)
		require.NoError(t, err)

		f := &Finder{
			State:       state,
			Payloads:    payloads,
			Collections: collections,
		}

		// fetch from database
		fetched, err := f.GetByHeightRange(4, 5)
		require.NoError(t, err)

		// check fetched correct data
		require.Len(t, fetched, 2)

		require.Len(t, fetched[0].Collections, 2)
		require.Len(t, fetched[1].Collections, 1)

		require.Len(t, fetched[0].Collections[0].Transactions, 1)
		require.Len(t, fetched[0].Collections[1].Transactions, 2)
		require.Len(t, fetched[1].Collections[0].Transactions, 3)

		require.Equal(t,
			fetched[0].Collections[0].Transactions[0].TxID,
			col1.Collection.Transactions[0].ID().String(),
		)

		// unhappy path: endHeight is lower than startHeight
		_, err = f.GetByHeightRange(5, 4)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "must be smaller")

		// unhapp path: range not exist
		snapNotFound := &mock.Snapshot{}
		snapNotFound.On("Head").Return(nil, storage.ErrNotFound)
		state.On("AtHeight", uint64(99998)).Return(snapNotFound, nil)
		_, err = f.GetByHeightRange(99998, 99999)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "could not find header by height 99998")

		// unhapp path: must not start from 0
		_, err = f.GetByHeightRange(0, 3)
		require.Error(t, err)
		require.Contains(t, fmt.Sprintf("%v", err), "start-height must not be 0")
	})
}
