package backend

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	testifyMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storageMock "github.com/onflow/flow-go/storage/mock"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// ScriptExecutorSuite is a test suite for testing the ScriptExecutor.
// It sets up the necessary components and dependencies for executing scripts.
type ScriptExecutorSuite struct {
	suite.Suite

	log            zerolog.Logger
	registerIndex  storage.RegisterIndex
	versionControl *version.VersionControl
	reporter       *syncmock.IndexReporter
	indexReporter  *index.Reporter
	scripts        *execution.Scripts
	chain          flow.Chain
	dbDir          string
	height         uint64
	headers        storage.Headers
	vm             *fvm.VirtualMachine
	vmCtx          fvm.Context
	snapshot       snapshot.SnapshotTree
}

// TestScriptExecutorSuite runs the ScriptExecutorSuite test suite.
func TestScriptExecutorSuite(t *testing.T) {
	suite.Run(t, new(ScriptExecutorSuite))
}

// newBlockHeadersStorage creates a mock block header storage for the given blocks.
func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByHeight := make(map[uint64]*flow.Block)
	for _, b := range blocks {
		blocksByHeight[b.Height] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByHeight(blocksByHeight))
}

// bootstrap initializes the virtual machine and updates the register index and snapshot.
// This method sets up the initial state for the virtual machine.
func (s *ScriptExecutorSuite) bootstrap() {
	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, out, err := s.vm.Run(
		s.vmCtx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		s.snapshot)

	// Ensure no errors occurred during the bootstrap process
	s.Require().NoError(err)
	s.Require().NoError(out.Err)

	// Update the block height and store the updated registers
	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	// Append the execution snapshot to the snapshot tree
	s.snapshot = s.snapshot.Append(executionSnapshot)
}

// SetupTest sets up the test environment for each test in the suite.
// This includes initializing various components and mock objects needed for the tests.
func (s *ScriptExecutorSuite) SetupTest() {
	lockManager := storage.NewTestingLockManager()
	s.log = unittest.Logger()
	s.chain = flow.Emulator.Chain()

	s.reporter = syncmock.NewIndexReporter(s.T())
	s.indexReporter = index.NewReporter()
	err := s.indexReporter.Initialize(s.reporter)
	require.NoError(s.T(), err)

	blockchain := unittest.BlockchainFixture(10)
	s.headers = newBlockHeadersStorage(blockchain)
	s.height = blockchain[0].Height

	protocolState := testutil.ProtocolStateWithSourceFixture(nil)

	s.snapshot = snapshot.NewSnapshotTree(nil)
	s.vm = fvm.NewVirtualMachine()
	s.vmCtx = fvm.NewContext(
		s.chain,
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	s.dbDir = unittest.TempDir(s.T())
	db := pebbleStorage.NewBootstrappedRegistersWithPathForTest(s.T(), s.dbDir, s.height, s.height)
	pebbleRegisters, err := pebbleStorage.NewRegisters(db, pebbleStorage.PruningDisabled)
	s.Require().NoError(err)
	s.registerIndex = pebbleRegisters

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	s.Require().NoError(err)

	indexerCore := indexer.New(
		s.log,
		module.ExecutionStateIndexerMetrics(metrics.NewNoopCollector()),
		nil,
		s.registerIndex,
		s.headers,
		nil,
		nil,
		nil,
		nil,
		nil,
		s.chain.ChainID(),
		derivedChainData,
		nil,
		metrics.NewNoopCollector(),
		lockManager,
		nil, // accountTxIndex
	)

	s.scripts = execution.NewScripts(
		s.log,
		metrics.NewNoopCollector(),
		s.chain.ChainID(),
		protocolState,
		s.headers,
		indexerCore.RegisterValue,
		query.NewDefaultConfig(),
		derivedChainData,
		true,
	)
	s.bootstrap()
}

// TearDownTest runs after each test finishes and ensures components are done before continuing.
func (s *ScriptExecutorSuite) TearDownTest() {
	unittest.RequireComponentsDoneBefore(s.T(), 100*time.Millisecond, s.versionControl)
}

// TestExecuteAtBlockHeight tests script execution at a specific block height.
// It verifies the behavior of script execution with and without version control.
func (s *ScriptExecutorSuite) TestExecuteAtBlockHeight() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	script := []byte("access(all) fun main() { }")
	var scriptArgs [][]byte
	var expectedResult = []byte("{\"type\":\"Void\"}\n")

	s.reporter.On("LowestIndexedHeight").Return(s.height, nil)

	// This test simulates the behavior when the version beacon is not set in the script executor,
	// but it should still work by omitting the version control checks.
	s.Run("test script execution without version control", func() {
		scriptExec := NewScriptExecutor(s.log, uint64(0), math.MaxUint64)
		s.reporter.On("HighestIndexedHeight").Return(s.height+1, nil).Once()

		// Initialize the script executor without version control
		err := scriptExec.Initialize(s.indexReporter, s.scripts, nil)
		s.Require().NoError(err)

		// Execute the script at the specified block height
		res, err := scriptExec.ExecuteAtBlockHeight(ctx, script, scriptArgs, s.height)
		s.Assert().NoError(err)
		s.Assert().NotNil(res)
		s.Assert().Equal(expectedResult, res)
	})

	// This test simulates the behavior when the version beacon is set in the script executor,
	// and the script is running on a block with a compatible version that matches the current node version.
	s.Run("test script execution with version control with compatible version", func() {
		// Set up a mock version beacons events storage
		versionBeacons := storageMock.NewVersionBeacons(s.T())
		versionEvents := map[uint64]*flow.SealedVersionBeacon{
			s.height: versionBeaconEventFixture(
				s.T(),
				s.height,
				[]uint64{s.height},
				[]string{"0.0.1"},
			),
		}
		// Mock the Highest method to return a version beacon with a specific version
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(mocks.StorageMapGetter(versionEvents))

		var err error
		// Initialize version control with the mock version beacons
		s.versionControl, err = version.NewVersionControl(
			s.log,
			versionBeacons,
			semver.New("0.0.1"),
			s.height-1,
			s.height,
		)
		require.NoError(s.T(), err)

		// Create a mock signaler context for testing
		ictx := irrecoverable.NewMockSignalerContext(s.T(), ctx)

		// Start the VersionControl component
		s.versionControl.Start(ictx)

		// Ensure the component is ready before proceeding
		unittest.RequireComponentsReadyBefore(s.T(), 2*time.Second, s.versionControl)

		// Initialize the script executor with version control
		scriptExec := NewScriptExecutor(s.log, uint64(0), math.MaxUint64)
		s.reporter.On("HighestIndexedHeight").Return(s.height+1, nil)

		err = scriptExec.Initialize(s.indexReporter, s.scripts, s.versionControl)
		s.Require().NoError(err)

		// Execute the script at the specified block height
		res, err := scriptExec.ExecuteAtBlockHeight(ctx, script, scriptArgs, s.height)
		s.Assert().NoError(err)
		s.Assert().NotNil(res)
		s.Assert().Equal(expectedResult, res)
	})

	// This test simulates the behavior when the version beacon is set in the script executor,
	// and the script is running on a block with incompatible version that mismatch the current node version.
	s.Run("test script execution with version control with incompatible version", func() {
		// Set up a mock version beacons events storage
		versionBeacons := storageMock.NewVersionBeacons(s.T())
		versionEvents := map[uint64]*flow.SealedVersionBeacon{
			s.height: versionBeaconEventFixture(
				s.T(),
				s.height,
				[]uint64{s.height},
				[]string{"0.0.2"},
			),
		}
		// Mock the Highest method to return a version beacon with a specific version
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(mocks.StorageMapGetter(versionEvents))

		var err error
		// Initialize version control with the mock version beacons
		s.versionControl, err = version.NewVersionControl(
			s.log,
			versionBeacons,
			semver.New("0.0.1"),
			s.height-1,
			s.height,
		)
		require.NoError(s.T(), err)

		// Create a mock signaler context for testing
		ictx := irrecoverable.NewMockSignalerContext(s.T(), ctx)

		// Start the VersionControl component
		s.versionControl.Start(ictx)

		// Ensure the component is ready before proceeding
		unittest.RequireComponentsReadyBefore(s.T(), 2*time.Second, s.versionControl)

		// Initialize the script executor with version control
		scriptExec := NewScriptExecutor(s.log, uint64(0), math.MaxUint64)
		s.reporter.On("HighestIndexedHeight").Return(s.height+1, nil)

		err = scriptExec.Initialize(s.indexReporter, s.scripts, s.versionControl)
		s.Require().NoError(err)

		// Execute the script at the specified block height
		res, err := scriptExec.ExecuteAtBlockHeight(ctx, script, scriptArgs, s.height)
		s.Assert().ErrorIs(ErrIncompatibleNodeVersion, err)
		s.Assert().Nil(res)
	})
}

// versionBeaconEventFixture creates a SealedVersionBeacon for the given heights and versions.
// This is used to simulate version events in the tests.
func versionBeaconEventFixture(
	t *testing.T,
	sealHeight uint64,
	heights []uint64,
	versions []string,
) *flow.SealedVersionBeacon {
	require.Equal(t, len(heights), len(versions), "the heights array should be the same length as the versions array")
	var vb []flow.VersionBoundary
	for i := 0; i < len(heights); i++ {
		vb = append(vb, flow.VersionBoundary{
			BlockHeight: heights[i],
			Version:     versions[i],
		})
	}

	return &flow.SealedVersionBeacon{
		VersionBeacon: unittest.VersionBeaconFixture(
			unittest.WithBoundaries(vb...),
		),
		SealHeight: sealHeight,
	}
}
