package fixtures

import (
	"fmt"
	"testing"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratorSuiteRandomSeed(t *testing.T) {
	// Test with random seed (no seed specified)
	suite1 := NewGeneratorSuite()
	suite2 := NewGeneratorSuite()

	// generated values should be different
	header := suite1.BlockHeaders().Fixture()
	header2 := suite2.BlockHeaders().Fixture()
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
				return a.BlockHeaders().Fixture(), b.BlockHeaders().Fixture()
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				return a.BlockHeaders().List(n), b.BlockHeaders().List(n)
			},
			sanity: func(t *testing.T, suite *GeneratorSuite) {
				// Test basic block header generation
				header1 := suite.BlockHeaders().Fixture()
				require.NotNil(t, header1)
				assert.Equal(t, flow.Emulator, header1.ChainID)
				assert.Greater(t, header1.Height, uint64(0))
				assert.Greater(t, header1.View, uint64(0))

				// Test with specific height
				header2 := suite.BlockHeaders().Fixture(suite.BlockHeaders().WithHeight(100))
				assert.Equal(t, uint64(100), header2.Height)

				// Test with parent details
				parent := suite.BlockHeaders().Fixture()
				child1 := suite.BlockHeaders().Fixture(suite.BlockHeaders().WithParent(parent.ID(), parent.View, parent.Height))
				assert.Equal(t, parent.Height+1, child1.Height)
				assert.Equal(t, parent.ID(), child1.ParentID)
				assert.Equal(t, parent.ChainID, child1.ChainID)
				assert.Less(t, parent.View, child1.View)

				// Test with parent header
				child2 := suite.BlockHeaders().Fixture(suite.BlockHeaders().WithParentHeader(parent))
				assert.Equal(t, parent.Height+1, child2.Height)
				assert.Equal(t, parent.ID(), child2.ParentID)
				assert.Equal(t, parent.ChainID, child2.ChainID)
				assert.Less(t, parent.View, child2.View)

				// Test on specific chain
				header3 := suite.BlockHeaders().Fixture(suite.BlockHeaders().WithChainID(flow.Testnet))
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
				opts := []func(*signerIndicesConfig){a.SignerIndices().WithSignerCount(1000, 5)}
				return a.SignerIndices().Fixture(opts...), b.SignerIndices().Fixture(opts...)
			},
			list: func(a, b *GeneratorSuite, n int) (any, any) {
				// use a larger total to avoid accidental collisions
				opts := []func(*signerIndicesConfig){a.SignerIndices().WithSignerCount(1000, 5)}
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

				qc2 := suite.QuorumCertificates().Fixture(suite.QuorumCertificates().WithView(100))
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

				col2 := suite.Collections().Fixture(suite.Collections().WithTxCount(10))
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

				tr2 := suite.TransactionResults().Fixture(suite.TransactionResults().WithErrorMessage("custom error"))
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

				ltr2 := suite.LightTransactionResults().Fixture(suite.LightTransactionResults().WithFailed(true))
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

				eventWithCCF := suite.Events().Fixture(suite.Events().WithEncoding(entities.EventEncodingVersion_CCF_V0))
				assert.NotEmpty(t, eventWithCCF.Payload)

				eventWithJSON := suite.Events().Fixture(suite.Events().WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0))
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

				eventType2 := suite.EventTypes().Fixture(suite.EventTypes().WithEventName("CustomEvent"))
				assert.Contains(t, string(eventType2), "CustomEvent")

				eventType3 := suite.EventTypes().Fixture(suite.EventTypes().WithContractName("CustomContract"))
				assert.Contains(t, string(eventType3), "CustomContract")

				addr := suite.Addresses().Fixture()
				eventType4 := suite.EventTypes().Fixture(suite.EventTypes().WithAddress(addr))
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
