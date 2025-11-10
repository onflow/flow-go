package systemcollection

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// IMPORTANT: Do not modify the expected IDs in this file! The IDs are hardcoded to ensure any changes
// to the system collection are intentional. If these tests are failing, one of the following is true:
//   - the data returned by the fixture generator has changed.
//   - one or more system or scheduled transaction has changed.
//
// If the fixture data has changed, verify this is the source of the failures by undoing only the
// generator changes and re-running the tests. If the test pass, then it is OK to update the expected
// IDs. If the test still fails, there may be a regression or unintended change to the system collection.
// Identify the source and fix the issue.
//
// When the system collection is intentionally changed, follow these steps create a new version:
//   1. create a new versioned system collection by copying the current latest version, and incrementing
//      the version number.
//   2. update the previous latest version by duplicating the previous logic from blueprints.
//   3. DO NOT modify this test file. It should continue to pass once step 2 is complete.
//   4. create a new test file for the new latest version by copying this file and updating the
//      expected IDs.
//   5. run all tests and ensure they pass.

const (
	// generatorSeedV0 is the prng seed used by the fixture generator. This ensures that every run
	// of the test suite will receive identical random data, which is required to ensure the entity
	// hashes are consistent.
	generatorSeedV0 = 1234
)

func TestBuilderV0Suite(t *testing.T) {
	suite.Run(t, new(builderV0Suite))
}

// builderV0Suite tests that the BuilderV0 implementation produces the expected system collection.
// It uses a deterministic seed for the fixture generator to ensure the random test data is consistent
// across runs.
type builderV0Suite struct {
	suite.Suite

	builder builderV0
	g       *fixtures.GeneratorSuite
}

func (s *builderV0Suite) SetupTest() {
	s.g = fixtures.NewGeneratorSuite(
		fixtures.WithSeed(generatorSeedV0),
		fixtures.WithChainID(flow.Mainnet),
	)
}

func (s *builderV0Suite) TestProcessCallbacksTransaction() {
	expectedID := flow.MustHexStringToIdentifier("a9caece21b073a85cdfa8e27c6781426025ab67d7018b9afe388a18cc293e14f")

	tx, err := s.builder.ProcessCallbacksTransaction(s.g.ChainID().Chain())
	s.Require().NoError(err)
	s.Require().True(expectedID == tx.ID(), "invalid change made in the v0 versioned system collection")
}

func (s *builderV0Suite) TestExecuteCallbacksTransactions() {
	events := s.g.PendingExecutionEvents().List(5)

	expectedIDs := []flow.Identifier{
		flow.MustHexStringToIdentifier("5bb0f5398ac2239ca9144dd3ed154607b476f31fc700ed977de93f9d69b846c0"),
		flow.MustHexStringToIdentifier("c8fd263ccc01b1886922d2250fd43f95ab4b50fa92391faae7d56e7c4006bcf6"),
		flow.MustHexStringToIdentifier("dc1177b8c77c7a7f2291d0ff3f5c1633117406080f20bae363aec34698f9187d"),
		flow.MustHexStringToIdentifier("60154fc6a81573209cec831ddca02eb46b21c9b3deac8163ae50a3246cf357c4"),
		flow.MustHexStringToIdentifier("77137f278b48af272bfa0c1d4029f2d0d3c10abe5444ceec96a34391c6bb5b53"),
	}

	txs, err := s.builder.ExecuteCallbacksTransactions(s.g.ChainID().Chain(), events)
	s.Require().NoError(err)
	s.Require().Len(txs, len(expectedIDs))
	for i, tx := range txs {
		s.Require().True(expectedIDs[i] == tx.ID(), "invalid change made in the v0 versioned system collection")
	}
}

func (s *builderV0Suite) TestSystemChunkTransaction() {
	expectedID := flow.MustHexStringToIdentifier("3408f8b1aa1b33cfc3f78c3f15217272807b14cec4ef64168bcf313bc4174621")

	tx, err := s.builder.SystemChunkTransaction(s.g.ChainID().Chain())
	s.Require().NoError(err)
	s.Require().True(expectedID == tx.ID(), "invalid change made in the v0 versioned system collection")
}

func (s *builderV0Suite) TestSystemCollection() {
	s.Run("with scheduled transactions", func() {
		events := s.g.PendingExecutionEvents().List(5)

		expectedID := flow.MustHexStringToIdentifier("a6b2717447152c1944e0a067571326aef7111caa2ea989ce2245a4278b1b2950")

		collection, err := s.builder.SystemCollection(s.g.ChainID().Chain(), access.StaticEventProvider(events))
		s.Require().NoError(err)
		s.Require().Len(collection.Transactions, 7)
		s.Require().True(expectedID == collection.ID(), "invalid change made in the v0 versioned system collection")
	})

	s.Run("without scheduled transactions - nil provider", func() {
		expectedID := flow.MustHexStringToIdentifier("920ae4b0e27bfc40ee29bd948f8238f3c15d68fc21e96df54abc92c4511e217b")

		collection, err := s.builder.SystemCollection(s.g.ChainID().Chain(), nil)
		s.Require().NoError(err)
		s.Require().Len(collection.Transactions, 2)
		s.Require().True(expectedID == collection.ID(), "invalid change made in the v0 versioned system collection")
	})

	s.Run("without scheduled transactions - empty events list", func() {
		expectedID := flow.MustHexStringToIdentifier("920ae4b0e27bfc40ee29bd948f8238f3c15d68fc21e96df54abc92c4511e217b")

		collection, err := s.builder.SystemCollection(s.g.ChainID().Chain(), access.StaticEventProvider(flow.EventsList{}))
		s.Require().NoError(err)
		s.Require().Len(collection.Transactions, 2)
		s.Require().True(expectedID == collection.ID(), "invalid change made in the v0 versioned system collection")
	})
}
