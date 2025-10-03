package fixtures

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/model/flow"
)

func TestGeneratorSuiteRandomSeed(t *testing.T) {
	// Test with random seed (no seed specified)
	suite1 := NewGeneratorSuite()
	suite2 := NewGeneratorSuite()

	// generated values should be different
	header := suite1.Headers().Fixture()
	header2 := suite2.Headers().Fixture()
	assert.NotEqual(t, header, header2)
}

func TestGeneratorsDeterminism(t *testing.T) {
	// Test all generators
	tests := []struct {
		name    string
		fixture func(a, b *GeneratorSuite) (any, any)
		list    func(a, b *GeneratorSuite, n int) (any, any)
		sanity  func(t *testing.T, suite *GeneratorSuite)
	}{
		// All generators have both Fixture and List methods
		{
			name: "BlockHeaders",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Headers().Fixture(), b.Headers().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Headers().List(n), b.Headers().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				// Test basic block header generation
				header1 := suite.Headers().Fixture()
				require.NotNil(t, header1)
				assert.Equal(t, flow.Emulator, header1.ChainID)
				assert.Greater(t, header1.Height, uint64(0))
				assert.Greater(t, header1.View, uint64(0))

				// Test with specific height
				header2 := suite.Headers().Fixture(Header.WithHeight(100))
				assert.Equal(t, uint64(100), header2.Height)

				// Test with parent details
				parent := suite.Headers().Fixture()
				child1 := suite.Headers().Fixture(Header.WithParent(parent.ID(), parent.View, parent.Height))
				assert.Equal(t, parent.Height+1, child1.Height)
				assert.Equal(t, parent.ID(), child1.ParentID)
				assert.Equal(t, parent.ChainID, child1.ChainID)
				assert.Less(t, parent.View, child1.View)

				// Test with parent header
				child2 := suite.Headers().Fixture(Header.WithParentHeader(parent))
				assert.Equal(t, parent.Height+1, child2.Height)
				assert.Equal(t, parent.ID(), child2.ParentID)
				assert.Equal(t, parent.ChainID, child2.ChainID)
				assert.Less(t, parent.View, child2.View)

				// Test on specific chain
				header3 := suite.Headers().Fixture(Header.WithChainID(flow.Testnet))
				assert.Equal(t, flow.Testnet, header3.ChainID)
			},
		},
		{
			name: "Time",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Time().Fixture(), b.Time().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Time().List(n), b.Time().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				// Test basic time generation
				time1 := suite.Time().Fixture()
				require.NotNil(t, time1)
				assert.Greater(t, time1.Unix(), int64(0))

				// Test default is random
				time2 := suite.Time().Fixture()
				assert.NotEqual(t, time1, time2)
			},
		},
		{
			name: "Identifiers",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Identifiers().Fixture(), b.Identifiers().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Identifiers().List(n), b.Identifiers().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				identifier := suite.Identifiers().Fixture()
				assert.NotEmpty(t, identifier)
				assert.NotEqual(t, flow.ZeroID, identifier)
			},
		},
		{
			name: "Signatures",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Signatures().Fixture(), b.Signatures().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Signatures().List(n), b.Signatures().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				signature := suite.Signatures().Fixture()
				assert.NotEmpty(t, signature)
				assert.Len(t, signature, crypto.SignatureLenBLSBLS12381)
			},
		},
		{
			name: "Addresses",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Addresses().Fixture(), b.Addresses().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Addresses().List(n), b.Addresses().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				addr := suite.Addresses().Fixture()
				assert.True(t, suite.ChainID().Chain().IsValid(addr))
			},
		},
		{
			name: "SignerIndices",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				// use a larger total to avoid accidental collisions
				opts := []SignerIndicesOption{SignerIndices.WithSignerCount(1000, 5)}
				return a.SignerIndices().Fixture(opts...), b.SignerIndices().Fixture(opts...)
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				// use a larger total to avoid accidental collisions
				opts := []SignerIndicesOption{SignerIndices.WithSignerCount(1000, 5)}
				return a.SignerIndices().List(n, opts...), b.SignerIndices().List(n, opts...)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				indices := suite.SignerIndices().Fixture()
				assert.NotEmpty(t, indices)
			},
		},
		{
			name: "QuorumCertificates",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.QuorumCertificates().Fixture(), b.QuorumCertificates().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.QuorumCertificates().List(n), b.QuorumCertificates().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				qc := suite.QuorumCertificates().Fixture()
				assert.NotEmpty(t, qc)

				qc2 := suite.QuorumCertificates().Fixture(QuorumCertificate.WithView(100))
				assert.Equal(t, uint64(100), qc2.View)
			},
		},
		{
			name: "ChunkExecutionDatas",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ChunkExecutionDatas().Fixture(), b.ChunkExecutionDatas().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ChunkExecutionDatas().List(n), b.ChunkExecutionDatas().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				ced := suite.ChunkExecutionDatas().Fixture()
				assert.NotEmpty(t, ced)
			},
		},
		{
			name: "BlockExecutionDatas",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.BlockExecutionDatas().Fixture(), b.BlockExecutionDatas().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.BlockExecutionDatas().List(n), b.BlockExecutionDatas().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				bed := suite.BlockExecutionDatas().Fixture()
				assert.NotEmpty(t, bed)
			},
		},
		{
			name: "BlockExecutionDataEntities",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.BlockExecutionDataEntities().Fixture(), b.BlockExecutionDataEntities().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.BlockExecutionDataEntities().List(n), b.BlockExecutionDataEntities().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				bedEntity := suite.BlockExecutionDataEntities().Fixture()
				assert.NotEmpty(t, bedEntity)
			},
		},
		{
			name: "Transactions",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Transactions().Fixture(), b.Transactions().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Transactions().List(n), b.Transactions().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				tx := suite.Transactions().Fixture()
				assert.NotEmpty(t, tx)
			},
		},
		{
			name: "Collections",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Collections().Fixture(), b.Collections().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Collections().List(n), b.Collections().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				col := suite.Collections().Fixture()
				assert.NotEmpty(t, col)

				col2 := suite.Collections().Fixture(Collection.WithTxCount(10))
				assert.Len(t, col2.Transactions, 10)
			},
		},
		{
			name: "TrieUpdates",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.TrieUpdates().Fixture(), b.TrieUpdates().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.TrieUpdates().List(n), b.TrieUpdates().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				trie := suite.TrieUpdates().Fixture()
				assert.NotEmpty(t, trie)
			},
		},
		{
			name: "TransactionResults",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.TransactionResults().Fixture(), b.TransactionResults().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.TransactionResults().List(n), b.TransactionResults().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				tr := suite.TransactionResults().Fixture()
				assert.NotEmpty(t, tr)

				tr2 := suite.TransactionResults().Fixture(TransactionResult.WithErrorMessage("custom error"))
				assert.Equal(t, "custom error", tr2.ErrorMessage)
			},
		},
		{
			name: "LightTransactionResults",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.LightTransactionResults().Fixture(), b.LightTransactionResults().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.LightTransactionResults().List(n), b.LightTransactionResults().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				ltr := suite.LightTransactionResults().Fixture()
				assert.NotEmpty(t, ltr)

				ltr2 := suite.LightTransactionResults().Fixture(LightTransactionResult.WithFailed(true))
				assert.True(t, ltr2.Failed)
			},
		},
		{
			name: "TransactionSignatures",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.TransactionSignatures().Fixture(), b.TransactionSignatures().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.TransactionSignatures().List(n), b.TransactionSignatures().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				ts := suite.TransactionSignatures().Fixture()
				assert.NotEmpty(t, ts)
			},
		},
		{
			name: "ProposalKeys",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ProposalKeys().Fixture(), b.ProposalKeys().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ProposalKeys().List(n), b.ProposalKeys().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				pk := suite.ProposalKeys().Fixture()
				assert.NotEmpty(t, pk)
			},
		},
		{
			name: "Events",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Events().Fixture(), b.Events().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Events().List(n), b.Events().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				event := suite.Events().Fixture()
				assert.NotEmpty(t, event)

				eventWithCCF := suite.Events().Fixture(Event.WithEncoding(entities.EventEncodingVersion_CCF_V0))
				assert.NotEmpty(t, eventWithCCF.Payload)

				eventWithJSON := suite.Events().Fixture(Event.WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0))
				assert.NotEmpty(t, eventWithJSON.Payload)

				// Test events for transaction
				txID := suite.Identifiers().Fixture()
				txEvents := suite.Events().ForTransaction(txID, 0, 3)
				assert.Len(t, txEvents, 3)
				for i, event := range txEvents {
					assert.Equal(t, txID, event.TransactionID)
					assert.Equal(t, uint32(0), event.TransactionIndex)
					assert.Equal(t, uint32(i), event.EventIndex)
				}
			},
		},
		{
			name: "EventTypes",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.EventTypes().Fixture(), b.EventTypes().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.EventTypes().List(n), b.EventTypes().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				eventType1 := suite.EventTypes().Fixture()
				assert.NotEmpty(t, eventType1)

				eventType2 := suite.EventTypes().Fixture(EventType.WithEventName("CustomEvent"))
				assert.Contains(t, string(eventType2), "CustomEvent")

				eventType3 := suite.EventTypes().Fixture(EventType.WithContractName("CustomContract"))
				assert.Contains(t, string(eventType3), "CustomContract")

				addr := suite.Addresses().Fixture()
				eventType4 := suite.EventTypes().Fixture(EventType.WithAddress(addr))
				assert.Contains(t, string(eventType4), addr.String())
			},
		},
		{
			name: "LedgerPaths",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.LedgerPaths().Fixture(), b.LedgerPaths().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.LedgerPaths().List(n), b.LedgerPaths().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				lp := suite.LedgerPayloads().Fixture()
				assert.NotEmpty(t, lp)
			},
		},
		{
			name: "LedgerPayloads",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.LedgerPayloads().Fixture(), b.LedgerPayloads().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.LedgerPayloads().List(n), b.LedgerPayloads().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				lp := suite.LedgerPayloads().Fixture()
				assert.NotEmpty(t, lp)
			},
		},
		{
			name: "LedgerValues",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.LedgerValues().Fixture(), b.LedgerValues().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.LedgerValues().List(n), b.LedgerValues().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				lv := suite.LedgerValues().Fixture()
				assert.NotEmpty(t, lv)
			},
		},
		{
			name: "Identities",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Identities().Fixture(), b.Identities().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Identities().List(n), b.Identities().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				identity := suite.Identities().Fixture()
				assert.NotEmpty(t, identity.NodeID)
				assert.NotEmpty(t, identity.Address)
				assert.NotNil(t, identity.StakingPubKey)
				assert.NotNil(t, identity.NetworkPubKey)

				// Test with specific role
				consensus := suite.Identities().Fixture(Identity.WithRole(flow.RoleConsensus))
				assert.Equal(t, flow.RoleConsensus, consensus.Role)

				// Test with all roles
				identities := suite.Identities().List(10, Identity.WithAllRoles())
				rolesSeen := make(map[flow.Role]bool)
				for _, id := range identities {
					rolesSeen[id.Role] = true
				}
				assert.Equal(t, len(rolesSeen), len(flow.Roles())) // Should see multiple roles
			},
		},
		{
			name: "StateCommitments",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.StateCommitments().Fixture(), b.StateCommitments().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.StateCommitments().List(n), b.StateCommitments().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				sc := suite.StateCommitments().Fixture()
				assert.Len(t, sc, 32)
				assert.NotEqual(t, flow.DummyStateCommitment, sc)

				// Test empty state
				empty := suite.StateCommitments().Fixture(StateCommitment.WithEmptyState())
				assert.Equal(t, flow.EmptyStateCommitment, empty)

				// Test special state
				hash, err := hash.ToHash(suite.Random().RandomBytes(32))
				require.NoError(t, err)
				actual := suite.StateCommitments().Fixture(StateCommitment.WithHash(hash))
				assert.Equal(t, flow.StateCommitment(hash), actual)
			},
		},
		{
			name: "AggregatedSignatures",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.AggregatedSignatures().Fixture(), b.AggregatedSignatures().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.AggregatedSignatures().List(n), b.AggregatedSignatures().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				as := suite.AggregatedSignatures().Fixture()
				assert.NotEmpty(t, as.VerifierSignatures)
				assert.NotEmpty(t, as.SignerIDs)
			},
		},
		{
			name: "TimeoutCertificates",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.TimeoutCertificates().Fixture(), b.TimeoutCertificates().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.TimeoutCertificates().List(n), b.TimeoutCertificates().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				tc := suite.TimeoutCertificates().Fixture()
				assert.Greater(t, tc.View, uint64(0))
				assert.NotEmpty(t, tc.NewestQCViews)
				assert.NotEmpty(t, tc.SignerIndices)
			},
		},
		{
			name: "ExecutionResults",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ExecutionResults().Fixture(), b.ExecutionResults().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ExecutionResults().List(n), b.ExecutionResults().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				er := suite.ExecutionResults().Fixture()
				assert.NotEmpty(t, er.PreviousResultID)
				assert.NotEmpty(t, er.BlockID)
				assert.NotEmpty(t, er.Chunks)
			},
		},
		{
			name: "ExecutionReceipts",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ExecutionReceipts().Fixture(), b.ExecutionReceipts().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ExecutionReceipts().List(n), b.ExecutionReceipts().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				er := suite.ExecutionReceipts().Fixture()
				assert.NotEmpty(t, er.ExecutorID)
				assert.NotNil(t, er.ExecutionResult)
				assert.NotEmpty(t, er.Spocks)
				assert.NotEmpty(t, er.ExecutorSignature)
			},
		},
		{
			name: "Chunks",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Chunks().Fixture(), b.Chunks().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Chunks().List(n), b.Chunks().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				chunk := suite.Chunks().Fixture()
				assert.NotEmpty(t, chunk.StartState)
				assert.NotEmpty(t, chunk.EventCollection)
				assert.NotEmpty(t, chunk.BlockID)
			},
		},
		{
			name: "ServiceEvents",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ServiceEvents().Fixture(), b.ServiceEvents().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ServiceEvents().List(n), b.ServiceEvents().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				se := suite.ServiceEvents().Fixture()
				assert.NotEmpty(t, se.Type)
				assert.NotNil(t, se.Event)

				// Test with specific type
				setup := suite.ServiceEvents().Fixture(ServiceEvent.WithType(flow.ServiceEventSetup))
				assert.Equal(t, flow.ServiceEventSetup, setup.Type)
				assert.IsType(t, &flow.EpochSetup{}, setup.Event)
			},
		},
		{
			name: "VersionBeacons",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.VersionBeacons().Fixture(), b.VersionBeacons().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.VersionBeacons().List(n), b.VersionBeacons().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				vb := suite.VersionBeacons().Fixture()
				assert.NotEmpty(t, vb.VersionBoundaries)
			},
		},
		{
			name: "ProtocolStateVersionUpgrades",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ProtocolStateVersionUpgrades().Fixture(), b.ProtocolStateVersionUpgrades().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ProtocolStateVersionUpgrades().List(n), b.ProtocolStateVersionUpgrades().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				psvu := suite.ProtocolStateVersionUpgrades().Fixture()
				assert.Greater(t, psvu.NewProtocolStateVersion, uint64(0))
			},
		},
		{
			name: "SetEpochExtensionViewCounts",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.SetEpochExtensionViewCounts().Fixture(), b.SetEpochExtensionViewCounts().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.SetEpochExtensionViewCounts().List(n), b.SetEpochExtensionViewCounts().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				seevc := suite.SetEpochExtensionViewCounts().Fixture()
				assert.Greater(t, seevc.Value, uint64(0))
			},
		},
		{
			name: "EjectNodes",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.EjectNodes().Fixture(), b.EjectNodes().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.EjectNodes().List(n), b.EjectNodes().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				en := suite.EjectNodes().Fixture()
				assert.NotEmpty(t, en.NodeID)
			},
		},
		{
			name: "EpochSetups",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.EpochSetups().Fixture(), b.EpochSetups().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.EpochSetups().List(n), b.EpochSetups().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				es := suite.EpochSetups().Fixture()
				assert.Greater(t, es.Counter, uint64(0))
				assert.NotEmpty(t, es.Participants)
				assert.NotEmpty(t, es.Assignments)
				assert.Greater(t, es.FinalView, es.FirstView)
			},
		},
		{
			name: "EpochCommits",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.EpochCommits().Fixture(), b.EpochCommits().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.EpochCommits().List(n), b.EpochCommits().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				ec := suite.EpochCommits().Fixture()
				assert.Greater(t, ec.Counter, uint64(0))
				assert.NotEmpty(t, ec.ClusterQCs)
				assert.NotNil(t, ec.DKGGroupKey)
				assert.NotEmpty(t, ec.DKGParticipantKeys)
			},
		},
		{
			name: "EpochRecovers",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.EpochRecovers().Fixture(), b.EpochRecovers().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.EpochRecovers().List(n), b.EpochRecovers().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				er := suite.EpochRecovers().Fixture()
				assert.Equal(t, er.EpochSetup.Counter, er.EpochCommit.Counter)
				assert.NotEmpty(t, er.EpochSetup.Participants)
				assert.NotEmpty(t, er.EpochCommit.ClusterQCs)
			},
		},
		{
			name: "QuorumCertificatesWithSignerIDs",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.QuorumCertificatesWithSignerIDs().Fixture(), b.QuorumCertificatesWithSignerIDs().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.QuorumCertificatesWithSignerIDs().List(n), b.QuorumCertificatesWithSignerIDs().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				qc := suite.QuorumCertificatesWithSignerIDs().Fixture()
				assert.Greater(t, qc.View, uint64(0))
				assert.NotEmpty(t, qc.BlockID)
				assert.NotEmpty(t, qc.SignerIDs)
			},
		},
		{
			name: "Blocks",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Blocks().Fixture(), b.Blocks().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Blocks().List(n), b.Blocks().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				block := suite.Blocks().Fixture()
				assert.NotNil(t, block)
				assert.NotNil(t, block.Payload)
				assert.Equal(t, flow.Emulator, block.ChainID)
				assert.Greater(t, block.Height, uint64(0))
				assert.Greater(t, block.View, uint64(0))

				// Test with specific height
				block2 := suite.Blocks().Fixture(Block.WithHeight(100))
				assert.Equal(t, uint64(100), block2.Height)

				// Test with specific view
				block3 := suite.Blocks().Fixture(Block.WithView(200))
				assert.Equal(t, uint64(200), block3.View)

				// Test with specific chain ID
				block4 := suite.Blocks().Fixture(Block.WithChainID(flow.Testnet))
				assert.Equal(t, flow.Testnet, block4.ChainID)
			},
		},
		{
			name: "Payloads",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Payloads().Fixture(), b.Payloads().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Payloads().List(n), b.Payloads().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				payload := suite.Payloads().Fixture()
				assert.NotNil(t, payload)

				// Test with specific seals
				seal := suite.Seals().Fixture()
				payload2 := suite.Payloads().Fixture(Payload.WithSeals(seal))
				assert.Len(t, payload2.Seals, 1)
				assert.Equal(t, seal, payload2.Seals[0])

				// Test with specific guarantees
				guarantee := suite.Guarantees().Fixture()
				payload3 := suite.Payloads().Fixture(Payload.WithGuarantees(guarantee))
				assert.Len(t, payload3.Guarantees, 1)
				assert.Equal(t, guarantee, payload3.Guarantees[0])
			},
		},
		{
			name: "Seals",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Seals().Fixture(), b.Seals().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Seals().List(n), b.Seals().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				seal := suite.Seals().Fixture()
				assert.NotNil(t, seal)
				assert.NotEmpty(t, seal.BlockID)
				assert.NotEmpty(t, seal.ResultID)
				assert.NotEmpty(t, seal.FinalState)

				// Test with specific block ID
				blockID := suite.Identifiers().Fixture()
				seal2 := suite.Seals().Fixture(Seal.WithBlockID(blockID))
				assert.Equal(t, blockID, seal2.BlockID)

				// Test with specific result ID
				resultID := suite.Identifiers().Fixture()
				seal3 := suite.Seals().Fixture(Seal.WithResultID(resultID))
				assert.Equal(t, resultID, seal3.ResultID)

				// Test with specific final state
				finalState := suite.StateCommitments().Fixture()
				seal4 := suite.Seals().Fixture(Seal.WithFinalState(finalState))
				assert.Equal(t, finalState, seal4.FinalState)
			},
		},
		{
			name: "CollectionGuarantees",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Guarantees().Fixture(), b.Guarantees().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.Guarantees().List(n), b.Guarantees().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				guarantee := suite.Guarantees().Fixture()
				assert.NotNil(t, guarantee)
				assert.NotEmpty(t, guarantee.CollectionID)
				assert.NotEmpty(t, guarantee.ReferenceBlockID)
				assert.NotEmpty(t, guarantee.SignerIndices)
				assert.NotEmpty(t, guarantee.Signature)

				// Test with specific collection ID
				collectionID := suite.Identifiers().Fixture()
				guarantee2 := suite.Guarantees().Fixture(Guarantee.WithCollectionID(collectionID))
				assert.Equal(t, collectionID, guarantee2.CollectionID)

				// Test with specific reference block ID
				refBlockID := suite.Identifiers().Fixture()
				guarantee3 := suite.Guarantees().Fixture(Guarantee.WithReferenceBlockID(refBlockID))
				assert.Equal(t, refBlockID, guarantee3.ReferenceBlockID)
			},
		},
		{
			name: "ExecutionReceiptStubs",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.ExecutionReceiptStubs().Fixture(), b.ExecutionReceiptStubs().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.ExecutionReceiptStubs().List(n), b.ExecutionReceiptStubs().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				stub := suite.ExecutionReceiptStubs().Fixture()
				assert.NotNil(t, stub)
				assert.NotEmpty(t, stub.ExecutorID)
				assert.NotEmpty(t, stub.ResultID)
				assert.NotEmpty(t, stub.Spocks)
				assert.NotEmpty(t, stub.ExecutorSignature)

				// Test with specific executor ID
				executorID := suite.Identifiers().Fixture()
				stub2 := suite.ExecutionReceiptStubs().Fixture(ExecutionReceiptStub.WithExecutorID(executorID))
				assert.Equal(t, executorID, stub2.ExecutorID)

				// Test with specific result ID
				resultID := suite.Identifiers().Fixture()
				stub3 := suite.ExecutionReceiptStubs().Fixture(ExecutionReceiptStub.WithResultID(resultID))
				assert.Equal(t, resultID, stub3.ResultID)
			},
		},
		{
			name: "Crypto",
			fixture: func(a, b *GeneratorSuite) (any, any) {
				return a.Crypto().PrivateKey(crypto.ECDSAP256), b.Crypto().PrivateKey(crypto.ECDSAP256)
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				keys1 := make([]crypto.PrivateKey, n)
				keys2 := make([]crypto.PrivateKey, n)
				for i := range n {
					keys1[i] = a.Crypto().PrivateKey(crypto.BLSBLS12381)
					keys2[i] = b.Crypto().PrivateKey(crypto.BLSBLS12381)
				}
				return keys1, keys2
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				// Test BLS key generation
				blsKey := suite.Crypto().PrivateKey(crypto.BLSBLS12381)
				assert.NotNil(t, blsKey)
				assert.Equal(t, crypto.BLSBLS12381, blsKey.Algorithm())

				// Test ECDSA key generation
				ecdsaKey := suite.Crypto().PrivateKey(crypto.ECDSAP256)
				assert.NotNil(t, ecdsaKey)
				assert.Equal(t, crypto.ECDSAP256, ecdsaKey.Algorithm())

				// Test with custom seed
				seed := suite.Random().RandomBytes(crypto.KeyGenSeedMinLen)
				seededKey := suite.Crypto().PrivateKey(crypto.BLSBLS12381, PrivateKey.WithSeed(seed))
				assert.NotNil(t, seededKey)
			},
		},
	}

	suite1 := NewGeneratorSuite(WithSeed(42))
	suite2 := NewGeneratorSuite(WithSeed(42))

	// IMPORTANT: these tests must not be run in parallel, or they will receive non-deterministic
	// random data and fail.

	// Run all deterministic tests first to ensure that the both of the generators are at the same
	// point in their random streams.
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Deterministic %s Fixture", tt.name), func(t *testing.T) {
			fixture1, fixture2 := tt.fixture(suite1, suite2)
			assert.Equal(t, fixture1, fixture2)
		})

		t.Run(fmt.Sprintf("Deterministic %s List", tt.name), func(t *testing.T) {
			count := 3
			list1, list2 := tt.list(suite1, suite2, count)
			assert.Len(t, list1, count)
			assert.Len(t, list2, count)
			assert.Equal(t, list1, list2)
		})
	}

	suite3 := NewGeneratorSuite()
	suite4 := NewGeneratorSuite()

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Non-Deterministic %s Fixture", tt.name), func(t *testing.T) {
			fixture1, fixture2 := tt.fixture(suite3, suite4)
			assert.NotEqual(t, fixture1, fixture2)
		})

		t.Run(fmt.Sprintf("Non-Deterministic %s List", tt.name), func(t *testing.T) {
			count := 3
			list1, list2 := tt.list(suite3, suite4, count)
			assert.Len(t, list1, count)
			assert.Len(t, list2, count)
			assert.NotEqual(t, list1, list2)
		})
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Sanity Check %s", tt.name), func(t *testing.T) {
			tt.sanity(t, suite3)
		})
	}
}
