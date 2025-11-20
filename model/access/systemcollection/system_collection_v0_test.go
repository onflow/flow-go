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
		flow.MustHexStringToIdentifier("1205195687b7b48b57765d96e95d2f63de71f65abd04d50ce10b3f565576d287"),
		flow.MustHexStringToIdentifier("1f8cb48ee75570537b52f6a12bc1f53cd71c1ab9afc14f0dd82f6fa9cb774c96"),
		flow.MustHexStringToIdentifier("07a6813a5a7cb5d7c0fcfba01937148c08e61ba070ee9b4ea0d62617c7685352"),
		flow.MustHexStringToIdentifier("1f8085602b449221727ee018bf9e2ee30423bc4db630404889ec858c8bcd8f3a"),
		flow.MustHexStringToIdentifier("ecab0dd859325a07076a1d7af380d08771c77f3e3c707bb7d39a18b1b77fcd3d"),
	}

	txs, err := s.builder.ExecuteCallbacksTransactions(s.g.ChainID().Chain(), events)
	s.Require().NoError(err)
	s.Require().Len(txs, len(expectedIDs))
	for i, tx := range txs {
		s.Require().True(expectedIDs[i] == tx.ID(), "invalid change made in the v0 versioned system collection, expected %s, got %s", expectedIDs[i], tx.ID())
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

		expectedID := flow.MustHexStringToIdentifier("7c490ce58ad80d4c7f4f45a6434666cdbe5d2bc3f9c27d86aa6027cfbbe2f492")

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
