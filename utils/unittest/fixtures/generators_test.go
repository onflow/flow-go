package fixtures

import (
	"testing"
	"time"

	"github.com/onflow/crypto"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestGeneratorSuite(t *testing.T) {
	// Test with explicit seed for deterministic results
	suite := NewGeneratorSuite(t, WithSeed(12345))

	// Test basic block header generation
	header1 := suite.BlockHeaders().Fixture(t)
	require.NotNil(t, header1)
	assert.Equal(t, flow.Emulator, header1.ChainID)
	assert.Greater(t, header1.Height, uint64(0))
	assert.Greater(t, header1.View, uint64(0))

	// Test with specific height
	header2 := suite.BlockHeaders().Fixture(t, suite.BlockHeaders().WithHeight(100))
	assert.Equal(t, uint64(100), header2.Height)

	// Test with parent
	parent := suite.BlockHeaders().Fixture(t)
	child := suite.BlockHeaders().Fixture(t, suite.BlockHeaders().WithParent(parent))
	assert.Equal(t, parent.Height+1, child.Height)
	assert.Equal(t, parent.ID(), child.ParentID)
	assert.Equal(t, parent.ChainID, child.ChainID)

	// Test on specific chain
	header3 := suite.BlockHeaders().Fixture(t, suite.BlockHeaders().WithChainID(flow.Testnet))
	assert.Equal(t, flow.Testnet, header3.ChainID)

	// Test primitive generators
	id1 := suite.Identifiers().Fixture(t)
	require.NotNil(t, id1)

	idList := suite.Identifiers().List(t, 3)
	assert.Len(t, idList, 3)

	sig1 := suite.Signatures().Fixture(t)
	require.NotNil(t, sig1)
	assert.Len(t, sig1, crypto.SignatureLenBLSBLS12381)

	sigList := suite.Signatures().List(t, 2)
	assert.Len(t, sigList, 2)

	addr1 := suite.Addresses().Fixture(t)
	require.NotNil(t, addr1)

	addr2 := suite.Addresses().Fixture(t, suite.Addresses().WithChainID(flow.Emulator))
	require.NotNil(t, addr2)

	addr3 := suite.Addresses().Fixture(t, suite.Addresses().ServiceAddress())
	require.NotNil(t, addr3)

	addr4 := CorruptAddress(t, suite.Addresses().Fixture(t))
	require.NotNil(t, addr4)

	// Test signer indices
	indices1 := suite.SignerIndices().Fixture(t, 4)
	require.NotNil(t, indices1)

	indices2 := suite.SignerIndices().ByIndices(t, []int{0, 2, 4})
	require.NotNil(t, indices2)

	indicesList := suite.SignerIndices().List(t, 3, 2)
	assert.Len(t, indicesList, 3)

	// Test quorum certificates
	qc1 := suite.QuorumCertificates().Fixture(t)
	require.NotNil(t, qc1)

	qc2 := suite.QuorumCertificates().Fixture(t, suite.QuorumCertificates().WithView(100))
	assert.Equal(t, uint64(100), qc2.View)

	qcList := suite.QuorumCertificates().List(t, 2)
	assert.Len(t, qcList, 2)

	// Test chunk execution data
	ced1 := suite.ChunkExecutionDatas().Fixture(t)
	require.NotNil(t, ced1)

	ced2 := suite.ChunkExecutionDatas().Fixture(t, suite.ChunkExecutionDatas().WithMinSize(100))
	assert.NotNil(t, ced2)

	cedList := suite.ChunkExecutionDatas().List(t, 2)
	assert.Len(t, cedList, 2)

	// Test block execution data
	bed1 := suite.BlockExecutionDatas().Fixture(t)
	require.NotNil(t, bed1)

	bed2 := suite.BlockExecutionDatas().Fixture(t, suite.BlockExecutionDatas().WithBlockID(suite.Identifiers().Fixture(t)))
	assert.NotNil(t, bed2)

	bedList := suite.BlockExecutionDatas().List(t, 2)
	assert.Len(t, bedList, 2)

	// Test transactions
	tx1 := suite.Transactions().Fixture(t)
	require.NotNil(t, tx1)

	tx2 := suite.Transactions().Fixture(t, suite.Transactions().WithGasLimit(100))
	assert.Equal(t, uint64(100), tx2.GasLimit)

	txList := suite.Transactions().List(t, 2)
	assert.Len(t, txList, 2)

	txComplete := suite.FullTransactions().Fixture(t)
	require.NotNil(t, txComplete)

	txCompleteList := suite.FullTransactions().List(t, 2)
	assert.Len(t, txCompleteList, 2)

	// Test collections
	col1 := suite.Collections().Fixture(t, 1)
	require.NotNil(t, col1)

	col2 := suite.Collections().Fixture(t, 3)
	assert.Len(t, col2.Transactions, 3)

	colList := suite.Collections().List(t, 2, 1)
	assert.Len(t, colList, 2)

	// Test trie updates
	trie1 := suite.TrieUpdates().Fixture(t)
	require.NotNil(t, trie1)

	trie2 := suite.TrieUpdates().Fixture(t, suite.TrieUpdates().WithNumPaths(5))
	assert.Len(t, trie2.Paths, 5)

	trieList := suite.TrieUpdates().List(t, 2)
	assert.Len(t, trieList, 2)

	// Test transaction results
	tr1 := suite.TransactionResults().Fixture(t)
	require.NotNil(t, tr1)

	tr2 := suite.TransactionResults().Fixture(t, suite.TransactionResults().WithErrorMessage("custom error"))
	assert.Equal(t, "custom error", tr2.ErrorMessage)

	trList := suite.TransactionResults().List(t, 2)
	assert.Len(t, trList, 2)

	_ = trie1 // Use trie1 as needed

	// Test light transaction results
	ltr1 := suite.LightTransactionResults().Fixture(t)
	require.NotNil(t, ltr1)

	ltr2 := suite.LightTransactionResults().Fixture(t, suite.LightTransactionResults().WithFailed(true))
	assert.True(t, ltr2.Failed)

	ltrList := suite.LightTransactionResults().List(t, 2)
	assert.Len(t, ltrList, 2)

	// Test transaction signatures
	ts1 := suite.TransactionSignatures().Fixture(t)
	require.NotNil(t, ts1)

	ts2 := suite.TransactionSignatures().Fixture(t, suite.TransactionSignatures().WithSignerIndex(5))
	assert.Equal(t, 5, ts2.SignerIndex)

	tsList := suite.TransactionSignatures().List(t, 2)
	assert.Len(t, tsList, 2)

	// Test proposal keys
	pk1 := suite.ProposalKeys().Fixture(t)
	require.NotNil(t, pk1)

	pk2 := suite.ProposalKeys().Fixture(t, suite.ProposalKeys().WithSequenceNumber(100))
	assert.Equal(t, uint64(100), pk2.SequenceNumber)

	pkList := suite.ProposalKeys().List(t, 2)
	assert.Len(t, pkList, 2)

	// Test events
	event1 := suite.Events().Fixture(t)
	require.NotNil(t, event1)

	event2 := suite.Events().Fixture(t, suite.Events().WithEventType("A.0x1.Test.Event"))
	assert.Equal(t, flow.EventType("A.0x1.Test.Event"), event2.Type)

	eventList := suite.Events().List(t, 2)
	assert.Len(t, eventList, 2)

	// Test events for transaction
	txID := suite.Identifiers().Fixture(t)
	txEvents := suite.Events().ForTransaction(t, txID, 0, 3)
	assert.Len(t, txEvents, 3)
	for i, event := range txEvents {
		assert.Equal(t, txID, event.TransactionID)
		assert.Equal(t, uint32(0), event.TransactionIndex)
		assert.Equal(t, uint32(i), event.EventIndex)
	}

	// Test events with encoding
	eventWithCCF := suite.Events().Fixture(t, suite.Events().WithEncoding(entities.EventEncodingVersion_CCF_V0))
	require.NotNil(t, eventWithCCF)
	require.NotEmpty(t, eventWithCCF.Payload)

	eventWithJSON := suite.Events().Fixture(t, suite.Events().WithEncoding(entities.EventEncodingVersion_JSON_CDC_V0))
	require.NotNil(t, eventWithJSON)
	require.NotEmpty(t, eventWithJSON.Payload)

	// Verify that different encodings produce different payloads
	assert.NotEqual(t, eventWithCCF.Payload, eventWithJSON.Payload)

	// Test event types
	eventType1 := suite.EventTypes().Fixture(t)
	require.NotNil(t, eventType1)
	assert.NotEmpty(t, string(eventType1))

	eventType2 := suite.EventTypes().Fixture(t, suite.EventTypes().WithEventName("CustomEvent"))
	assert.Contains(t, string(eventType2), "CustomEvent")

	eventType3 := suite.EventTypes().Fixture(t, suite.EventTypes().WithContractName("CustomContract"))
	assert.Contains(t, string(eventType3), "CustomContract")

	eventType4 := suite.EventTypes().Fixture(t, suite.EventTypes().WithAddress(suite.Addresses().Fixture(t)))
	require.NotNil(t, eventType4)
}

func TestGeneratorSuiteRandomSeed(t *testing.T) {
	// Test with random seed (no seed specified)
	suite1 := NewGeneratorSuite(t)
	suite2 := NewGeneratorSuite(t)

	// generated values should be different
	header := suite1.BlockHeaders().Fixture(t)
	header2 := suite2.BlockHeaders().Fixture(t)
	assert.NotEqual(t, header, header2)
}

func TestGeneratorsDeterministic(t *testing.T) {
	// Test that primitive generators produce same results with same seed
	suite1 := NewGeneratorSuite(t, WithSeed(42))
	suite2 := NewGeneratorSuite(t, WithSeed(42))

	// Test block headers
	header1 := suite1.BlockHeaders().Fixture(t)
	header2 := suite2.BlockHeaders().Fixture(t)
	assert.Equal(t, header1, header2)

	// Test identifiers
	id1 := suite1.Identifiers().Fixture(t)
	id2 := suite2.Identifiers().Fixture(t)
	assert.Equal(t, id1, id2)

	// Test signatures
	sig1 := suite1.Signatures().Fixture(t)
	sig2 := suite2.Signatures().Fixture(t)
	assert.Equal(t, sig1, sig2)

	// Test addresses
	addr1 := suite1.Addresses().Fixture(t)
	addr2 := suite2.Addresses().Fixture(t)
	assert.Equal(t, addr1, addr2)

	// Test signer indices
	indices1 := suite1.SignerIndices().Fixture(t, 4)
	indices2 := suite2.SignerIndices().Fixture(t, 4)
	assert.Equal(t, indices1, indices2)

	// Test quorum certificates
	qc1 := suite1.QuorumCertificates().Fixture(t)
	qc2 := suite2.QuorumCertificates().Fixture(t)
	assert.Equal(t, qc1, qc2)

	// Test chunk execution data
	ced1 := suite1.ChunkExecutionDatas().Fixture(t)
	ced2 := suite2.ChunkExecutionDatas().Fixture(t)
	assert.Equal(t, ced1, ced2)

	// Test block execution data
	bed1 := suite1.BlockExecutionDatas().Fixture(t)
	bed2 := suite2.BlockExecutionDatas().Fixture(t)
	assert.Equal(t, bed1, bed2)

	// Test transactions
	tx1 := suite1.Transactions().Fixture(t)
	tx2 := suite2.Transactions().Fixture(t)
	assert.Equal(t, tx1, tx2)

	// Test collections
	col1 := suite1.Collections().Fixture(t, 1)
	col2 := suite2.Collections().Fixture(t, 1)
	assert.Equal(t, col1, col2)

	// Test trie updates
	trie1 := suite1.TrieUpdates().Fixture(t)
	trie2 := suite2.TrieUpdates().Fixture(t)
	assert.Equal(t, trie1, trie2)

	// Test transaction results
	tr1 := suite1.TransactionResults().Fixture(t)
	tr2 := suite2.TransactionResults().Fixture(t)
	assert.Equal(t, tr1, tr2)

	// Test transaction signatures
	ts1 := suite1.TransactionSignatures().Fixture(t)
	ts2 := suite2.TransactionSignatures().Fixture(t)
	assert.Equal(t, ts1, ts2)

	// Test proposal keys
	pk1 := suite1.ProposalKeys().Fixture(t)
	pk2 := suite2.ProposalKeys().Fixture(t)
	assert.Equal(t, pk1, pk2)

	// Test lists
	idList1 := suite1.Identifiers().List(t, 3)
	idList2 := suite2.Identifiers().List(t, 3)
	assert.Equal(t, idList1, idList2)

	sigList1 := suite1.Signatures().List(t, 2)
	sigList2 := suite2.Signatures().List(t, 2)
	assert.Equal(t, sigList1, sigList2)

	// Test events
	event1 := suite1.Events().Fixture(t)
	event2 := suite2.Events().Fixture(t)
	assert.Equal(t, event1, event2)

	// Test time generator
	time1 := suite1.Time().Fixture(t)
	time2 := suite2.Time().Fixture(t)
	assert.Equal(t, time1, time2)

	// Test time with specific base time
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	time3 := suite1.Time().Fixture(t, suite1.Time().WithBaseTime(baseTime))
	time4 := suite2.Time().Fixture(t, suite2.Time().WithBaseTime(baseTime))
	assert.Equal(t, time3, time4)

	// Test ledger path generator
	path1 := suite1.LedgerPaths().Fixture(t)
	path2 := suite2.LedgerPaths().Fixture(t)
	assert.Equal(t, path1, path2)

	paths1 := suite1.LedgerPaths().List(t, 3)
	paths2 := suite2.LedgerPaths().List(t, 3)
	assert.Equal(t, paths1, paths2)

	// Test ledger payload generator
	payload1 := suite1.LedgerPayloads().Fixture(t)
	payload2 := suite2.LedgerPayloads().Fixture(t)
	assert.Equal(t, payload1, payload2)

	payloads1 := suite1.LedgerPayloads().List(t, 2)
	payloads2 := suite2.LedgerPayloads().List(t, 2)
	assert.Equal(t, payloads1, payloads2)

	// Test ledger value generator
	value1 := suite1.LedgerValues().Fixture(t)
	value2 := suite2.LedgerValues().Fixture(t)
	assert.Equal(t, value1, value2)

	values1 := suite1.LedgerValues().List(t, 3)
	values2 := suite2.LedgerValues().List(t, 3)
	assert.Equal(t, values1, values2)
}
