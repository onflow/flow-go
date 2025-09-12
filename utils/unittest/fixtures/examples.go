package fixtures

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Example usage of the new generator system

// ExampleNewGeneratorSuite shows how to create a generator suite with a random seed
func ExampleNewGeneratorSuite() {
	// Create a suite with a random seed (no seed specified)
	suite := NewGeneratorSuite()

	// Generate a block header
	header := suite.BlockHeaders().Fixture()
	_ = header // avoid unused warnings
}

// ExampleNewGeneratorSuiteStaticSeed shows how to create a generator suite with a specific seed
func ExampleNewGeneratorSuiteStaticSeed() {
	// Create a suite with a specific seed for deterministic results
	suite := NewGeneratorSuite(WithSeed(12345))

	// Generate a block header
	header := suite.BlockHeaders().Fixture()
	_ = header // avoid unused warnings
}

// ExampleBlockHeaderGenerator shows various ways to generate block headers
func ExampleBlockHeaderGenerator() {
	suite := NewGeneratorSuite()

	headerGen := suite.BlockHeaders()

	// Basic fixture
	header := headerGen.Fixture()

	// With specific height
	headerWithHeight := headerGen.Fixture(headerGen.WithHeight(100))

	// With parent
	parent := headerGen.Fixture()
	child1 := headerGen.Fixture(headerGen.WithParent(parent.ID(), parent.View, parent.Height))
	child2 := headerGen.Fixture(headerGen.WithParentHeader(parent))

	// On specific chain
	headerOnChain := headerGen.Fixture(headerGen.WithChainID(flow.Testnet))

	// With multiple options
	headerWithOptions := headerGen.Fixture(
		headerGen.WithHeight(200),
		headerGen.WithView(300),
	)

	// avoid unused warnings
	_ = header
	_ = headerWithHeight
	_ = child1
	_ = child2
	_ = headerOnChain
	_ = headerWithOptions
}

// ExampleAllGenerators shows usage of all available generators
func ExampleAllGenerators() {
	suite := NewGeneratorSuite()

	// Basic fixture
	header1 := suite.BlockHeaders().Fixture()

	// With specific height
	headerGen := suite.BlockHeaders()
	header2 := headerGen.Fixture(headerGen.WithHeight(100))

	// With parent
	parent := headerGen.Fixture()
	child1 := headerGen.Fixture(headerGen.WithParent(parent.ID(), parent.View, parent.Height))
	child2 := headerGen.Fixture(headerGen.WithParentHeader(parent))

	// On specific chain
	header3 := headerGen.Fixture(headerGen.WithChainID(flow.Testnet))

	// With options
	header4 := headerGen.Fixture(headerGen.WithHeight(200), headerGen.WithView(300))

	// Test primitive generators
	id1 := suite.Identifiers().Fixture()
	idList := suite.Identifiers().List(5)

	sig1 := suite.Signatures().Fixture()
	sigList := suite.Signatures().List(3)

	addGen := suite.Addresses()
	addr1 := addGen.Fixture()
	addr2 := addGen.Fixture(addGen.WithChainID(flow.Emulator))
	addr3 := addGen.Fixture(addGen.ServiceAddress())
	addr4 := CorruptAddress(addGen.Fixture(), flow.Testnet)

	// Test signer indices generator
	indicesGen := suite.SignerIndices()
	indices1 := indicesGen.Fixture(indicesGen.WithSignerCount(10, 4))
	indices2 := indicesGen.Fixture(indicesGen.WithIndices(10, []int{0, 2, 4}))
	indicesList := indicesGen.List(3, indicesGen.WithSignerCount(10, 2))

	// Test quorum certificate generator
	qcGen := suite.QuorumCertificates()
	qc1 := qcGen.Fixture()
	qc2 := qcGen.Fixture(qcGen.WithView(100), qcGen.WithBlockID(suite.Identifiers().Fixture()))
	qc3 := qcGen.Fixture(qcGen.CertifiesBlock(header1))
	qcList := qcGen.List(3)

	// Test chunk execution data generator
	cedGen := suite.ChunkExecutionDatas()
	ced1 := cedGen.Fixture()
	ced2 := cedGen.Fixture(cedGen.WithMinSize(100))
	cedList := cedGen.List(3)

	// Test block execution data generator
	bedGen := suite.BlockExecutionDatas()
	bed1 := bedGen.Fixture()
	bed2 := bedGen.Fixture(bedGen.WithBlockID(suite.Identifiers().Fixture()))
	bedList := bedGen.List(2)

	// Test transaction generator
	txGen := suite.Transactions()
	tx1 := txGen.Fixture()
	tx2 := txGen.Fixture(txGen.WithGasLimit(100))
	txList := txGen.List(3)

	// Test collection generator
	colGen := suite.Collections()
	col1 := colGen.Fixture(colGen.WithTxCount(1))
	col2 := colGen.Fixture(colGen.WithTxCount(3))
	colList := colGen.List(2, colGen.WithTxCount(3))

	// Test trie update generator
	trieGen := suite.TrieUpdates()
	trie1 := trieGen.Fixture()
	trie2 := trieGen.Fixture(trieGen.WithNumPaths(5))
	trieList := trieGen.List(2)

	// Test transaction result generator
	trGen := suite.TransactionResults()
	tr1 := trGen.Fixture()
	tr2 := trGen.Fixture(trGen.WithErrorMessage("custom error"))
	trList := trGen.List(2)

	// Test light transaction result generator
	ltrGen := suite.LightTransactionResults()
	ltr1 := ltrGen.Fixture()
	ltr2 := ltrGen.Fixture(ltrGen.WithFailed(true))
	ltrList := ltrGen.List(2)

	// Test transaction signature generator
	tsGen := suite.TransactionSignatures()
	ts1 := tsGen.Fixture()
	ts2 := tsGen.Fixture(tsGen.WithSignerIndex(5))
	tsList := tsGen.List(2)

	// Test proposal key generator
	pkGen := suite.ProposalKeys()
	pk1 := pkGen.Fixture()
	pk2 := pkGen.Fixture(pkGen.WithSequenceNumber(100))
	pkList := pkGen.List(2)

	// Test event generator
	eventGen := suite.Events()
	event1 := eventGen.Fixture()
	event2 := eventGen.Fixture(eventGen.WithEventType("A.0x1.Test.Event"))
	eventList := eventGen.List(2)

	// Test events for transaction
	txID := suite.Identifiers().Fixture()
	txEvents := eventGen.ForTransaction(txID, 0, 3)

	// Test event type generator
	eventTypeGen := suite.EventTypes()
	eventType1 := eventTypeGen.Fixture()
	eventType2 := eventTypeGen.Fixture(eventTypeGen.WithEventName("CustomEvent"))
	eventType3 := eventTypeGen.Fixture(eventTypeGen.WithContractName("CustomContract"))
	eventType4 := eventTypeGen.Fixture(eventTypeGen.WithAddress(suite.Addresses().Fixture()))

	// avoid unused warnings
	_ = header1
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
	_ = child1
	_ = child2
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
	suite := NewGeneratorSuite()

	randomGen := suite.Random()

	// Generate random bytes
	bytes := randomGen.RandomBytes(32)

	// Generate random string
	str := randomGen.RandomString(16)

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

	// Generate random unsigned integers less than n
	uintVal := randomGen.Uintn(100)
	uint32Val := randomGen.Uint32n(100)
	uint64Val := randomGen.Uint64n(100)

	// Generate random number in the specified inclusive range
	numInRange1 := randomGen.IntInRange(1, 100)
	numInRange2 := InclusiveRange(randomGen, 1, 100)

	// Generate random number in the specified inclusive range with a specific type
	numInRangeWithType1 := randomGen.Uint32InRange(1, 100)
	numInRangeWithType2 := InclusiveRange[uint32](randomGen, 1, 100)

	// Generate random signed integers in inclusive ranges
	intInRange := randomGen.IntInRange(-50, 50)
	int32InRange := randomGen.Int32InRange(-25, 25)
	int64InRange := randomGen.Int64InRange(-100, 100)

	// Generate random unsigned integers in inclusive ranges
	uintInRange := randomGen.UintInRange(10, 90)
	uint32InRange := randomGen.Uint32InRange(5, 95)
	uint64InRange := randomGen.Uint64InRange(1, 1000)

	// Select random element from a slice
	slice := []string{"apple", "banana", "cherry", "date"}
	randomElement := RandomElement(randomGen, slice)

	// Select random element from a slice of integers
	intSlice := []int{1, 2, 3, 4, 5}
	randomInt := RandomElement(randomGen, intSlice)

	// avoid unused warnings
	_ = bytes
	_ = str
	_ = val32
	_ = val64
	_ = int32Val
	_ = int64Val
	_ = intVal
	_ = uintVal
	_ = uint32Val
	_ = uint64Val
	_ = numInRange1
	_ = numInRange2
	_ = numInRangeWithType1
	_ = numInRangeWithType2
	_ = intInRange
	_ = int32InRange
	_ = int64InRange
	_ = uintInRange
	_ = uint32InRange
	_ = uint64InRange
	_ = randomElement
	_ = randomInt
}

// ExampleTimeGenerator shows usage of the time generator methods
func ExampleTimeGenerator() {
	suite := NewGeneratorSuite()

	timeGen := suite.Time()

	// Basic time fixture
	time1 := timeGen.Fixture()

	// Time with specific base time
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	time2 := timeGen.Fixture(timeGen.WithBaseTime(baseTime))

	// Time with specific offset
	time3 := timeGen.Fixture(timeGen.WithOffset(time.Hour))

	// Time with random offset
	time4 := timeGen.Fixture(timeGen.WithOffsetRandom(24 * time.Hour))

	// Time in a specific timezone
	time5 := timeGen.Fixture(timeGen.WithTimezone(time.FixedZone("EST", -5*3600)))

	// Time with multiple options
	time6 := timeGen.Fixture(
		timeGen.WithBaseTime(baseTime),
		timeGen.WithOffset(time.Hour),
		timeGen.WithTimezone(time.FixedZone("EST", -5*3600)),
	)

	// avoid unused warnings
	_ = time1
	_ = time2
	_ = time3
	_ = time4
	_ = time5
	_ = time6
}

// ExampleLedgerPathGenerator shows usage of the ledger path generator methods
func ExampleLedgerPathGenerator() {
	suite := NewGeneratorSuite()

	pathGen := suite.LedgerPaths()

	// Basic path fixture
	path1 := pathGen.Fixture()

	// List of paths
	paths := pathGen.List(3)

	// avoid unused warnings
	_ = path1
	_ = paths
}

// ExampleLedgerPayloadGenerator shows usage of the ledger payload generator methods
func ExampleLedgerPayloadGenerator() {
	suite := NewGeneratorSuite()

	payloadGen := suite.LedgerPayloads()

	// Basic payload fixture
	payload1 := payloadGen.Fixture()

	// Payload with specific size
	payload2 := payloadGen.Fixture(payloadGen.WithSize(4, 16))

	// Payload with specific value
	value := suite.LedgerValues().Fixture()
	payload3 := payloadGen.Fixture(payloadGen.WithValue(value))

	// List of payloads
	payloads := payloadGen.List(3)

	// avoid unused warnings
	_ = payload1
	_ = payload2
	_ = payload3
	_ = payloads
}

// ExampleLedgerValueGenerator shows usage of the ledger value generator methods
func ExampleLedgerValueGenerator() {
	suite := NewGeneratorSuite()

	valueGen := suite.LedgerValues()

	// Basic value fixture
	value1 := valueGen.Fixture()

	// Value with specific size
	value2 := valueGen.Fixture(valueGen.WithSize(4, 16))

	// List of values
	values := valueGen.List(3)

	// List of values with specific size
	largeValues := valueGen.List(2, valueGen.WithSize(8, 32))

	// avoid unused warnings
	_ = value1
	_ = value2
	_ = values
	_ = largeValues
}
