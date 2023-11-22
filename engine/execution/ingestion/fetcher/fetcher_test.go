package fetcher_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/fetcher"
	"github.com/onflow/flow-go/model/flow"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	statemock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

var _ ingestion.CollectionFetcher = (*fetcher.CollectionFetcher)(nil)

func TestFetch(t *testing.T) {
	// prepare data
	blockID := unittest.IdentifierFixture()
	height := uint64(10)
	nodes := unittest.IdentityListFixture(10)
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.ReferenceBlockID = unittest.IdentifierFixture()
	signers, err := signature.EncodeSignersToIndices(nodes.NodeIDs(), flow.IdentifierList{nodes[0].ID(), nodes[1].ID()})
	require.NoError(t, err)
	guarantee.SignerIndices = signers

	// mock depedencies
	cluster := new(statemock.Cluster)
	cluster.On("Members").Return(nodes)
	epoch := new(statemock.Epoch)
	epoch.On("ClusterByChainID", guarantee.ChainID).Return(cluster, nil)
	epochs := new(statemock.EpochQuery)
	epochs.On("Current").Return(epoch)
	snapshot := new(statemock.Snapshot)
	snapshot.On("Epochs").Return(epochs)
	state := new(statemock.State)
	state.On("AtBlockID", guarantee.ReferenceBlockID).Return(snapshot).Times(1)

	request := new(modulemock.Requester)
	var filter flow.IdentityFilter
	request.On("EntityByID", guarantee.CollectionID, mock.AnythingOfType("flow.IdentityFilter")).Run(
		func(args mock.Arguments) {
			filter = args.Get(1).(flow.IdentityFilter)
		},
	).Return().Times(1)

	// create fetcher
	fetcher := fetcher.NewCollectionFetcher(unittest.Logger(), request, state, false)

	// fetch collections
	err = fetcher.FetchCollection(blockID, height, guarantee)
	require.NoError(t, err)

	request.AssertExpectations(t)
	state.AssertExpectations(t)

	// verify request.EntityByID is called with the right filter
	for i, signer := range nodes {
		isSigner := i == 0 || i == 1
		require.Equal(t, isSigner, filter(signer))
	}
}
