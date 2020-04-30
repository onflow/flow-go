package consensus

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	module "github.com/dapperlabs/flow-go/module/mock"
	networkmock "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// when there is no block finalized, it should use genesis block to bootstrap

func WithParticipant(t *testing.T) {
	// create consensus node participants
	consensus := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))

	// create non-consensus nodes
	collection := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	verification := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	execution := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// append additional nodes to consensus
	participants := append(flow.IdentityList{}, consensus, collection, verification, execution)

	// add all identities to genesis block and
	// create and bootstrap consensus node with the genesis
	genesis := run.GenerateRootBlock(participants, run.GenerateRootSeal([]byte{}))

	db, _ := unittest.TempBadgerDB(t)
	state, err := protocol.NewState(db)
	require.NoError(t, err)

	err = state.Mutate().Bootstrap(&genesis)
	require.NoError(t, err)

	localID := consensus.ID()

	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	notifier := notifications.NewLogConsumer(log)

	// initialize no-op metrics mock
	metrics := &module.Metrics{}
	metrics.On("HotStuffBusyDuration", mock.Anything, mock.Anything)
	metrics.On("HotStuffIdleDuration", mock.Anything, mock.Anything)
	metrics.On("HotStuffWaitDuration", mock.Anything, mock.Anything)

	// make local
	priv := helper.MakeBLSKey(t)
	local, err := local.New(consensus, priv)
	require.NoError(t, err)

	headersDB := storage.NewHeaders(db)
	payloadsDB := storage.NewPayloads(db)
	blocksDB := storage.NewBlocks(db)
	viewsDB := storage.NewViews(db)

	guarantees, err := stdmap.NewGuarantees(1000)
	require.NoError(t, err)
	seals, err := stdmap.NewSeals(1000)
	require.NoError(t, err)

	// initialize the block builder
	build := builder.NewBuilder(db, guarantees, seals)

	signer := &module.Signer{}
	net := &module.Network{}

	// initialize the pending blocks cache
	cache := buffer.NewPendingBlocks()

	rootHeader := &genesis.Header
	rootQC := &model.QuorumCertificate{
		View:      genesis.View,
		BlockID:   genesis.ID(),
		SignerIDs: nil, // TODO
		SigData:   nil,
	}

	com, err := committee.NewMainConsensusCommitteeState(state, localID)
	require.NoError(t, err)

	// initialize the block finalizer
	final := finalizer.NewFinalizer(db, guarantees, seals)

	prov := &networkmock.Engine{}

	// initialize the compliance engine
	comp, err := compliance.New(log, net, local, state, headersDB, payloadsDB, prov, cache)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synchronization.New(log, net, local, state, blocksDB, comp)
	require.NoError(t, err)

	// initialize the block finalizer
	hot, err := NewParticipant(log, notifier, metrics, headersDB,
		viewsDB, com, state, build, final, signer, comp, rootHeader,
		rootQC, WithTimeout(hotstuffTimeout))

	require.NoError(t, err)

	comp = comp.WithSynchronization(sync).WithConsensus(hot)
}
func TestInit(t *testing.T) {

}

// when there is block finalized, it should use the finalized block to recovery the forks
func TestRecover(t *testing.T) {
}
