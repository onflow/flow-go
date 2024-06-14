package backend

import (
	"context"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"math"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	testifyMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/computation/query/mock"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

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
}

func TestScriptExecutorSuite(t *testing.T) {
	suite.Run(t, new(ScriptExecutorSuite))
}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByHeight := make(map[uint64]*flow.Block)
	for _, b := range blocks {
		blocksByHeight[b.Header.Height] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByHeight(blocksByHeight))
}

func (s *ScriptExecutorSuite) SetupTest() {
	s.log = unittest.Logger()
	s.chain = flow.Emulator.Chain()

	s.reporter = syncmock.NewIndexReporter(s.T())
	s.indexReporter = index.NewReporter()
	err := s.indexReporter.Initialize(s.reporter)
	require.NoError(s.T(), err)

	entropyProvider := testutil.EntropyProviderFixture(nil)
	blockchain := unittest.BlockchainFixture(10)
	headers := newBlockHeadersStorage(blockchain)
	s.height = blockchain[0].Header.Height

	entropyBlock := mock.NewEntropyProviderPerBlock(s.T())
	entropyBlock.
		On("AtBlockID", testifyMock.AnythingOfType("flow.Identifier")).
		Return(entropyProvider).
		Maybe()

	sealedHeader := unittest.BlockHeaderWithParentFixture(blockchain[len(blockchain)-1].Header) // height 11

	// Set up a mock version beacons storage.
	versionBeacons := storageMock.NewVersionBeacons(s.T())
	// Mock the Highest method to return a version beacon with a specific version.
	versionBeacons.
		On("Highest", testifyMock.AnythingOfType("uint64")).
		Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
			if height == sealedHeader.Height {
				return &flow.SealedVersionBeacon{
					VersionBeacon: unittest.VersionBeaconFixture(
						unittest.WithBoundaries(
							flow.VersionBoundary{
								BlockHeight: sealedHeader.Height,
								Version:     "0.0.1",
							}),
					),
					SealHeight: sealedHeader.Height,
				}, nil
			} else {
				return nil, nil
			}
		}, nil)

	s.versionControl = version.NewVersionControl(
		s.log,
		versionBeacons,
		semver.New("0.0.1"),
		blockchain[0].Header.Height,
		sealedHeader.Height,
	)

	s.dbDir = unittest.TempDir(s.T())
	db := pebbleStorage.NewBootstrappedRegistersWithPathForTest(s.T(), s.dbDir, s.height, s.height)
	pebbleRegisters, err := pebbleStorage.NewRegisters(db)
	s.Require().NoError(err)
	s.registerIndex = pebbleRegisters

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	s.Require().NoError(err)

	indexerCore, err := indexer.New(
		s.log,
		metrics.NewNoopCollector(),
		nil,
		s.registerIndex,
		headers,
		nil,
		nil,
		nil,
		nil,
		s.chain,
		derivedChainData,
		nil,
	)
	s.Require().NoError(err)

	s.scripts = execution.NewScripts(
		s.log,
		metrics.NewNoopCollector(),
		s.chain.ChainID(),
		entropyBlock,
		headers,
		indexerCore.RegisterValue,
		query.NewDefaultConfig(),
		derivedChainData,
		true,
	)
}

func (s *ScriptExecutorSuite) TestExecuteAtBlockHeight() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scriptExec := NewScriptExecutor(s.log, s.versionControl, uint64(0), math.MaxUint64)

	err := scriptExec.Initialize(s.indexReporter, s.scripts)
	s.Require().NoError(err)

	script := []byte("access(all) fun main() { return 1 }")
	arguments := [][]byte{[]byte("arg1"), []byte("arg2")}
	_, err = scriptExec.ExecuteAtBlockHeight(ctx, script, arguments, 11)
	s.Require().NoError(err)
}
