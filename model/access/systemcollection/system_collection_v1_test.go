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
	// generatorSeedV1 is the prng seed used by the fixture generator. This ensures that every run
	// of the test suite will receive identical random data, which is required to ensure the entity
	// hashes are consistent.
	generatorSeedV1 = 1234
)

func TestBuilderV1Suite(t *testing.T) {
	suite.Run(t, new(builderV1Suite))
}

// builderV1Suite tests that the BuilderV1 implementation produces the expected system collection.
// It uses a deterministic seed for the fixture generator to ensure the random test data is consistent
// across runs.
type builderV1Suite struct {
	suite.Suite

	builder builderV1
	g       *fixtures.GeneratorSuite
}

func (s *builderV1Suite) SetupTest() {
	s.g = fixtures.NewGeneratorSuite(
		fixtures.WithSeed(generatorSeedV1),
		fixtures.WithChainID(flow.Mainnet),
	)
}

func (s *builderV1Suite) TestProcessCallbacksTransaction() {
	expectedID := flow.MustHexStringToIdentifier("a9caece21b073a85cdfa8e27c6781426025ab67d7018b9afe388a18cc293e14f")

	tx, err := s.builder.ProcessCallbacksTransaction(s.g.ChainID().Chain())
	s.Require().NoError(err)
	s.Require().Equal(expectedID, tx.ID())
}

func (s *builderV1Suite) TestExecuteCallbacksTransactions() {
	events := s.g.PendingExecutionEvents().List(5)

	expectedIDs := []flow.Identifier{
		flow.MustHexStringToIdentifier("26631afae9b2f00bc587e30f139f20f66597be629ad8e716b49a9e528aaaf662"),
		flow.MustHexStringToIdentifier("6d95c33df610685b8657c60f3ab282e125e4fe30a48039f76dfe767d482bb630"),
		flow.MustHexStringToIdentifier("c3bea89b3c1b50a83655e4e353837d646580b33ee46b98fe22e0e17d70d4c238"),
		flow.MustHexStringToIdentifier("87eb99714e988000b3e5e501317e737317377bb878cf4080048e50b5219f68fb"),
		flow.MustHexStringToIdentifier("72784d50599e0e5923d2b1530027490885f12d165a13c6e07baee81a98ad2a41"),
	}

	txs, err := s.builder.ExecuteCallbacksTransactions(s.g.ChainID().Chain(), events)
	s.Require().NoError(err)
	s.Require().Len(txs, len(expectedIDs))
	for i, tx := range txs {
		s.Require().Equal(expectedIDs[i], tx.ID())
	}
}

func (s *builderV1Suite) TestSystemChunkTransaction() {
	expectedID := flow.MustHexStringToIdentifier("3408f8b1aa1b33cfc3f78c3f15217272807b14cec4ef64168bcf313bc4174621")

	tx, err := s.builder.SystemChunkTransaction(s.g.ChainID().Chain())
	s.Require().NoError(err)
	s.Require().Equal(expectedID, tx.ID())
}

func (s *builderV1Suite) TestSystemCollection() {
	s.Run("with scheduled transactions", func() {
		events := s.g.PendingExecutionEvents().List(5)

		expectedID := flow.MustHexStringToIdentifier("ab56922a5eb884139b1d4ba9c5abb22a0ae9c050b81e461e5f25399cde9e9de4")

		collection, err := s.builder.SystemCollection(s.g.ChainID().Chain(), access.StaticEventProvider(events))
		s.Require().NoError(err)
		s.Require().Len(collection.Transactions, 7)
		s.Require().Equal(expectedID, collection.ID())
	})

	s.Run("without scheduled transactions - nil provider", func() {
		expectedID := flow.MustHexStringToIdentifier("920ae4b0e27bfc40ee29bd948f8238f3c15d68fc21e96df54abc92c4511e217b")

		collection, err := s.builder.SystemCollection(s.g.ChainID().Chain(), nil)
		s.Require().NoError(err)
		s.Require().Len(collection.Transactions, 2)
		s.Require().Equal(expectedID, collection.ID())
	})

	s.Run("without scheduled transactions - empty events list", func() {
		expectedID := flow.MustHexStringToIdentifier("920ae4b0e27bfc40ee29bd948f8238f3c15d68fc21e96df54abc92c4511e217b")

		collection, err := s.builder.SystemCollection(s.g.ChainID().Chain(), access.StaticEventProvider(flow.EventsList{}))
		s.Require().NoError(err)
		s.Require().Len(collection.Transactions, 2)
		s.Require().Equal(expectedID, collection.ID())
	})
}
