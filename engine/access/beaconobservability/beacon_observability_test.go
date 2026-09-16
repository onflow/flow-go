package beaconobservability

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClassifySigType(t *testing.T) {
	t.Run("staking-only signature", func(t *testing.T) {
		sigData := unittest.SignatureFixture()
		require.Len(t, sigData, signature.SigLen)

		sigType, err := classifySigType(sigData)
		require.NoError(t, err)
		require.Equal(t, sigTypeStaking, sigType)
	})

	t.Run("combined staking + beacon signature", func(t *testing.T) {
		stakingSig := unittest.SignatureFixture()
		beaconSig := unittest.SignatureFixture()
		sigData := append(stakingSig, beaconSig...)
		require.Len(t, sigData, 2*signature.SigLen)

		sigType, err := classifySigType(sigData)
		require.NoError(t, err)
		require.Equal(t, sigTypeBeacon, sigType)
	})

	t.Run("unexpected length returns error", func(t *testing.T) {
		sigData := make([]byte, signature.SigLen+1)

		_, err := classifySigType(sigData)
		require.Error(t, err)
		require.ErrorIs(t, err, signature.ErrInvalidSignatureFormat)
	})
}

func TestBeaconObservabilityCollector(t *testing.T) {
	registry := prometheus.NewRegistry()
	collector := metrics.NewBeaconObservabilityCollector(registry)

	nodeID := unittest.IdentifierFixture()

	collector.NetworkFinalizedInitProposerMetrics(nodeID)
	collector.NetworkFinalizedProposerSigType(nodeID, sigTypeBeacon)
	collector.NetworkFinalizedProposerSigType(nodeID, sigTypeBeacon)
	collector.NetworkFinalizedProposerSigType(nodeID, sigTypeStaking)
	collector.NetworkFinalizedLastProposalHeight(nodeID, 42)
	collector.NetworkDKGBeaconThreshold(6)

	family, err := registry.Gather()
	require.NoError(t, err)

	metricMap := make(map[string]float64)
	for _, fam := range family {
		for _, m := range fam.Metric {
			labels := labelString(m.GetLabel())
			key := fam.GetName() + labels
			metricMap[key] = m.GetCounter().GetValue()
			if m.GetGauge() != nil {
				metricMap[key] = m.GetGauge().GetValue()
			}
		}
	}

	require.Equal(t, 2.0, metricMap["network_finalized_proposer_sig_type_total{nodeid="+nodeID.String()+",type=beacon}"])
	require.Equal(t, 1.0, metricMap["network_finalized_proposer_sig_type_total{nodeid="+nodeID.String()+",type=staking}"])
	require.Equal(t, 42.0, metricMap["network_finalized_last_proposal_height{nodeid="+nodeID.String()+"}"])
	require.Equal(t, 6.0, metricMap["network_finalized_dkg_beacon_threshold"])

	collector.NetworkFinalizedDeleteProposerMetrics(nodeID)

	family, err = registry.Gather()
	require.NoError(t, err)

	for _, fam := range family {
		if fam.GetName() == "network_finalized_proposer_sig_type_total" ||
			fam.GetName() == "network_finalized_last_proposal_height" {
			for _, m := range fam.Metric {
				for _, label := range m.GetLabel() {
					if label.GetName() == "nodeid" && label.GetValue() == nodeID.String() {
						t.Fatalf("metric series for deleted node should not exist: %s", fam.GetName())
					}
				}
			}
		}
	}
}

func labelString(labels []*dto.LabelPair) string {
	if len(labels) == 0 {
		return ""
	}
	result := "{"
	for i, label := range labels {
		if i > 0 {
			result += ","
		}
		result += label.GetName() + "=" + label.GetValue()
	}
	result += "}"
	return result
}

// mockMetrics is a minimal in-memory implementation of module.BeaconObservabilityMetrics for testing
// the component without Prometheus.
type mockMetrics struct {
	proposerSigTypes    map[flow.Identifier]map[string]uint64
	lastProposalHeights map[flow.Identifier]uint64
	beaconThreshold     uint64
	deletedNodeIDs      []flow.Identifier
	initializedNodeIDs  []flow.Identifier
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{
		proposerSigTypes:    make(map[flow.Identifier]map[string]uint64),
		lastProposalHeights: make(map[flow.Identifier]uint64),
	}
}

func (m *mockMetrics) NetworkFinalizedProposerSigType(nodeID flow.Identifier, sigType string) {
	if m.proposerSigTypes[nodeID] == nil {
		m.proposerSigTypes[nodeID] = make(map[string]uint64)
	}
	m.proposerSigTypes[nodeID][sigType]++
}

func (m *mockMetrics) NetworkFinalizedLastProposalHeight(nodeID flow.Identifier, height uint64) {
	m.lastProposalHeights[nodeID] = height
}

func (m *mockMetrics) NetworkDKGBeaconThreshold(threshold uint64) {
	m.beaconThreshold = threshold
}

func (m *mockMetrics) NetworkFinalizedDeleteProposerMetrics(nodeID flow.Identifier) {
	m.deletedNodeIDs = append(m.deletedNodeIDs, nodeID)
	delete(m.proposerSigTypes, nodeID)
	delete(m.lastProposalHeights, nodeID)
}

func (m *mockMetrics) NetworkFinalizedInitProposerMetrics(nodeID flow.Identifier) {
	m.initializedNodeIDs = append(m.initializedNodeIDs, nodeID)
}

func newTestComponent(t *testing.T, state *protocolmock.State, blocks *storagemock.Blocks, metrics *mockMetrics) *BeaconObservability {
	var component *BeaconObservability
	var err error
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		processedHeight, initErr := store.NewConsumerProgress(pebbleimpl.ToDB(db), "test-beacon-obs").Initialize(0)
		require.NoError(t, initErr)

		component, err = New(zerolog.Nop(), state, blocks, metrics, processedHeight)
	})
	require.NoError(t, err)
	return component
}

func TestProcessFinalizedBlockJob(t *testing.T) {
	state := protocolmock.NewState(t)
	snapshot := protocolmock.NewSnapshot(t)
	epochQuery := protocolmock.NewEpochQuery(t)
	committedEpoch := protocolmock.NewCommittedEpoch(t)
	dkg := protocolmock.NewDKG(t)

	state.On("Final").Return(snapshot).Maybe()
	snapshot.On("Epochs").Return(epochQuery)
	epochQuery.On("Current").Return(committedEpoch, nil)
	committedEpoch.On("DKG").Return(dkg, nil)
	dkg.On("Size").Return(uint(11))
	committedEpoch.On("InitialIdentities").Return(flow.IdentitySkeletonList{}, nil)

	blocks := storagemock.NewBlocks(t)
	metrics := newMockMetrics()
	component := newTestComponent(t, state, blocks, metrics)

	nodeID := unittest.IdentifierFixture()
	block := unittest.FullBlockFixture()
	block.HeaderBody.ProposerID = nodeID

	proposal := &flow.Proposal{
		Block:           *block,
		ProposerSigData: append(unittest.SignatureFixture(), unittest.SignatureFixture()...),
	}

	blocks.On("ProposalByHeight", block.Height).Return(proposal, nil)

	ctx := irrecoverable.NewMockSignalerContext(t, t.Context())
	job := jobqueue.BlockToJob(block)
	doneCalled := false
	done := func() { doneCalled = true }

	component.processFinalizedBlockJob(ctx, job, done)

	require.True(t, doneCalled)
	require.Equal(t, uint64(1), metrics.proposerSigTypes[nodeID][sigTypeBeacon])
	require.Equal(t, block.Height, metrics.lastProposalHeights[nodeID])
}

func TestUpdateCommitteeMembership(t *testing.T) {
	state := protocolmock.NewState(t)
	snapshot := protocolmock.NewSnapshot(t)
	epochQuery := protocolmock.NewEpochQuery(t)
	committedEpoch := protocolmock.NewCommittedEpoch(t)
	dkg := protocolmock.NewDKG(t)

	state.On("Final").Return(snapshot)
	snapshot.On("Epochs").Return(epochQuery)
	epochQuery.On("Current").Return(committedEpoch, nil)
	committedEpoch.On("DKG").Return(dkg, nil)
	dkg.On("Size").Return(uint(11))

	nodeA := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nodeB := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	nodeC := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	identities := flow.IdentitySkeletonList{
		&nodeA.IdentitySkeleton,
		&nodeB.IdentitySkeleton,
		&nodeC.IdentitySkeleton,
	}

	committedEpoch.On("InitialIdentities").Return(identities, nil)

	blocks := storagemock.NewBlocks(t)
	metrics := newMockMetrics()
	component := newTestComponent(t, state, blocks, metrics)

	require.Equal(t, uint64(6), metrics.beaconThreshold) // RandomBeaconThreshold(11)+1 = 5+1
	require.ElementsMatch(t, []flow.Identifier{nodeA.NodeID, nodeB.NodeID}, metrics.initializedNodeIDs)
	require.Len(t, component.committee, 2)
	require.Contains(t, component.committee, nodeA.NodeID)
	require.Contains(t, component.committee, nodeB.NodeID)
	require.NotContains(t, component.committee, nodeC.NodeID)
}
