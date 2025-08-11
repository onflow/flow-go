package fixtures

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Example usage of the new generator system

// ExampleNewGeneratorSuite shows how to create a generator suite with a random seed
func ExampleNewGeneratorSuite() {
	t := &testing.T{}

	// Create a suite with a random seed (no seed specified)
	suite := NewGeneratorSuite(t)

	// Generate a block header
	header := suite.BlockHeaders().Fixture(t)
	_ = header // Use header as needed
}

// ExampleNewGeneratorSuiteStaticSeed shows how to create a generator suite with a specific seed
func ExampleNewGeneratorSuiteStaticSeed() {
	t := &testing.T{}

	// Create a suite with a specific seed for deterministic results
	suite := NewGeneratorSuite(t, WithSeed(12345))

	// Generate a block header
	header := suite.BlockHeaders().Fixture(t)
	_ = header // Use header as needed
}

// ExampleBlockHeaderGenerator shows various ways to generate block headers
func ExampleBlockHeaderGenerator() {
	t := &testing.T{}
	suite := NewGeneratorSuite(t)

	headerGen := suite.BlockHeaders()

	// Basic fixture
	header := headerGen.Fixture(t)

	// With specific height
	headerWithHeight := headerGen.Fixture(t, headerGen.WithHeight(100))

	// With parent
	parent := headerGen.Fixture(t)
	child := headerGen.Fixture(t, headerGen.WithParent(parent))

	// On specific chain
	headerOnChain := headerGen.Fixture(t, headerGen.WithChainID(flow.Testnet))

	// With multiple options
	headerWithOptions := headerGen.Fixture(t,
		headerGen.WithHeight(200),
		headerGen.WithView(300),
	)

	_ = header
	_ = headerWithHeight
	_ = child
	_ = headerOnChain
	_ = headerWithOptions
}

// ExampleAllGenerators shows usage of all available generators
func ExampleAllGenerators() {
	t := &testing.T{}
	suite := NewGeneratorSuite(t)

	// Basic fixture
	header1 := suite.BlockHeaders().Fixture(t)

	// With specific height
	headerGen := suite.BlockHeaders()
	header2 := headerGen.Fixture(t, headerGen.WithHeight(100))

	// With parent
	parent := headerGen.Fixture(t)
	child := headerGen.Fixture(t, headerGen.WithParent(parent))

	// On specific chain
	header3 := headerGen.Fixture(t, headerGen.WithChainID(flow.Testnet))

	// With options
	header4 := headerGen.Fixture(t, headerGen.WithHeight(200), headerGen.WithView(300))

	// Test primitive generators
	id1 := suite.Identifiers().Fixture(t)
	idList := suite.Identifiers().List(t, 5)

	sig1 := suite.Signatures().Fixture(t)
	sigList := suite.Signatures().List(t, 3)

	addGen := suite.Addresses()
	addr1 := addGen.Fixture(t)
	addr2 := addGen.Fixture(t, addGen.WithChainID(flow.Emulator))
	addr3 := addGen.Fixture(t, addGen.ServiceAddress())
	addr4 := CorruptAddress(t, addGen.Fixture(t))

	// Test signer indices generator
	indicesGen := suite.SignerIndices()
	indices1 := indicesGen.Fixture(t, 4)
	indices2 := indicesGen.ByIndices(t, []int{0, 2, 4})
	indicesList := indicesGen.List(t, 3, 2)

	// Test quorum certificate generator
	qcGen := suite.QuorumCertificates()
	qc1 := qcGen.Fixture(t)
	qc2 := qcGen.Fixture(t, qcGen.WithView(100), qcGen.WithBlockID(suite.Identifiers().Fixture(t)))
	qc3 := qcGen.Fixture(t, qcGen.CertifiesBlock(header1))
	qcList := qcGen.List(t, 3)

	// Test chunk execution data generator
	cedGen := suite.ChunkExecutionDatas()
	ced1 := cedGen.Fixture(t)
	ced2 := cedGen.Fixture(t, cedGen.WithMinSize(100))
	cedList := cedGen.List(t, 3)

	// Test block execution data generator
	bedGen := suite.BlockExecutionDatas()
	bed1 := bedGen.Fixture(t)
	bed2 := bedGen.Fixture(t, bedGen.WithBlockID(suite.Identifiers().Fixture(t)))
	bedList := bedGen.List(t, 2)

	// Test transaction generator
	txGen := suite.Transactions()
	tx1 := txGen.Fixture(t)
	tx2 := txGen.Fixture(t, txGen.WithGasLimit(100))
	txList := txGen.List(t, 3)

	// Test full transaction generator
	fullTxGen := suite.FullTransactions()
	txComplete := fullTxGen.Fixture(t)
	txCompleteList := fullTxGen.List(t, 2)

	// Test collection generator
	colGen := suite.Collections()
	col1 := colGen.Fixture(t, 1)
	col2 := colGen.Fixture(t, 3)
	colList := colGen.List(t, 2, 1)

	// Test trie update generator
	trieGen := suite.TrieUpdates()
	trie1 := trieGen.Fixture(t)
	trie2 := trieGen.Fixture(t, trieGen.WithNumPaths(5))
	trieList := trieGen.List(t, 2)

	// Test transaction result generator
	trGen := suite.TransactionResults()
	tr1 := trGen.Fixture(t)
	tr2 := trGen.Fixture(t, trGen.WithErrorMessage("custom error"))
	trList := trGen.List(t, 2)

	// Test light transaction result generator
	ltrGen := suite.LightTransactionResults()
	ltr1 := ltrGen.Fixture(t)
	ltr2 := ltrGen.Fixture(t, ltrGen.WithFailed(true))
	ltrList := ltrGen.List(t, 2)

	// Test transaction signature generator
	tsGen := suite.TransactionSignatures()
	ts1 := tsGen.Fixture(t)
	ts2 := tsGen.Fixture(t, tsGen.WithSignerIndex(5))
	tsList := tsGen.List(t, 2)

	// Test proposal key generator
	pkGen := suite.ProposalKeys()
	pk1 := pkGen.Fixture(t)
	pk2 := pkGen.Fixture(t, pkGen.WithSequenceNumber(100))
	pkList := pkGen.List(t, 2)

	// Test event generator
	eventGen := suite.Events()
	event1 := eventGen.Fixture(t)
	event2 := eventGen.Fixture(t, eventGen.WithEventType("A.0x1.Test.Event"))
	eventList := eventGen.List(t, 2)

	// Test events for transaction
	txID := suite.Identifiers().Fixture(t)
	txEvents := eventGen.ForTransaction(t, txID, 0, 3)

	// Test event type generator
	eventTypeGen := suite.EventTypes()
	eventType1 := eventTypeGen.Fixture(t)
	eventType2 := eventTypeGen.Fixture(t, eventTypeGen.WithEventName("CustomEvent"))
	eventType3 := eventTypeGen.Fixture(t, eventTypeGen.WithContractName("CustomContract"))
	eventType4 := eventTypeGen.Fixture(t, eventTypeGen.WithAddress(suite.Addresses().Fixture(t)))

	_ = header1 // Use headers as needed
	_ = indices1
	_ = indices2
	_ = indicesList
	_ = qc1
	_ = qc2
	_ = qc3
	_ = qcList
	_ = ced1
	_ = ced2
	_ = cedList
	_ = bed1
	_ = bed2
	_ = bedList
	_ = tx1
	_ = tx2
	_ = txList
	_ = txComplete
	_ = txCompleteList
	_ = col1
	_ = col2
	_ = colList
	_ = trie1
	_ = trie2
	_ = trieList
	_ = tr1
	_ = tr2
	_ = trList
	_ = ltr1
	_ = ltr2
	_ = ltrList
	_ = ts1
	_ = ts2
	_ = tsList
	_ = pk1
	_ = pk2
	_ = pkList
	_ = event1
	_ = event2
	_ = eventList
	_ = txEvents
	_ = eventType1
	_ = eventType2
	_ = eventType3
	_ = eventType4
	_ = header2
	_ = child
	_ = header3
	_ = header4
	_ = id1
	_ = idList
	_ = sig1
	_ = sigList
	_ = addr1
	_ = addr2
	_ = addr3
	_ = addr4
}

// ExampleRandomGenerator shows usage of the random generator methods
func ExampleRandomGenerator() {
	t := &testing.T{}
	suite := NewGeneratorSuite(t)

	randomGen := suite.Random()

	// Generate random bytes
	bytes := randomGen.RandomBytes(t, 32)

	// Generate random string
	str := randomGen.RandomString(16)

	// Generate random number in range
	num := randomGen.Uint64InRange(1, 100)

	// Generate random uint32
	val32 := randomGen.Uint32()

	// Generate random uint64
	val64 := randomGen.Uint64()

	// Generate random int32
	int32Val := randomGen.Int31()

	// Generate random int64
	int64Val := randomGen.Int63()

	// Generate random int in range [0, n)
	intVal := randomGen.Intn(100)

	_ = bytes
	_ = str
	_ = num
	_ = val32
	_ = val64
	_ = int32Val
	_ = int64Val
	_ = intVal
}

// ExampleTimeGenerator shows usage of the time generator methods
func ExampleTimeGenerator() {
	t := &testing.T{}
	suite := NewGeneratorSuite(t)

	timeGen := suite.Time()

	// Basic time fixture
	time1 := timeGen.Fixture(t)

	// Time with specific base time
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	time2 := timeGen.Fixture(t, timeGen.WithBaseTime(baseTime))

	// Time with specific offset
	time3 := timeGen.Fixture(t, timeGen.WithOffset(time.Hour))

	// Time with random offset
	time4 := timeGen.Fixture(t, timeGen.WithOffsetRandom(24*time.Hour))

	// Time in a specific timezone
	time5 := timeGen.Fixture(t, timeGen.WithTimezone(time.FixedZone("EST", -5*3600)))

	// Time with multiple options
	time6 := timeGen.Fixture(t,
		timeGen.WithBaseTime(baseTime),
		timeGen.WithOffset(time.Hour),
		timeGen.WithTimezone(time.FixedZone("EST", -5*3600)),
	)

	_ = time1
	_ = time2
	_ = time3
	_ = time4
	_ = time5
	_ = time6
}
