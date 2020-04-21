package integration_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type Views struct {
	sync.Mutex
	latest uint64
}

func (v *Views) Store(action uint8, view uint64) error {
	v.Lock()
	defer v.Unlock()
	v.latest = view
	return nil
}

func (v *Views) Retrieve(action uint8) (uint64, error) {
	return v.latest, nil
}

type Signer struct {
	localID flow.Identifier
}

func (*Signer) CreateProposal(block *model.Block) (*model.Proposal, error) {
	proposal := &model.Proposal{
		Block:   block,
		SigData: nil,
	}
	return proposal, nil
}
func (s *Signer) CreateVote(block *model.Block) (*model.Vote, error) {
	vote := &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: s.localID,
		SigData:  nil,
	}
	return vote, nil
}
func (*Signer) CreateQC(votes []*model.Vote) (*model.QuorumCertificate, error) {
	voterIDs := make([]flow.Identifier, 0, len(votes))
	for _, vote := range votes {
		voterIDs = append(voterIDs, vote.SignerID)
	}
	qc := &model.QuorumCertificate{
		View:      votes[0].View,
		BlockID:   votes[0].BlockID,
		SignerIDs: voterIDs,
		SigData:   nil,
	}
	return qc, nil
}

func (*Signer) VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

func (*Signer) VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

type Conduit struct{}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return nil
}

type Network struct{}

func (n *Network) Register(code uint8, engine network.Engine) (network.Conduit, error) {
	return &Conduit{}, nil
}

type Finalizer struct{}

func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {
	return nil
}

type Communicator struct{}

func (c *Communicator) BroadcastProposal(p *flow.Header) error {
	return nil
}

func (c *Communicator) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {
	return nil
}

type Node struct {
	db         *badger.DB
	dbDir      string
	id         *flow.Identity
	compliance *compliance.Engine
	sync       *synchronization.Engine
	hot        *hotstuff.EventLoop
	headers    *storage.Headers
}

func createNodes(t *testing.T, n int) []*Node {
	participants := make([]*flow.Identity, 0)
	for i := 0; i < n; i++ {
		identity := unittest.IdentityFixture()
		participants = append(participants, identity)
	}

	collection := unittest.IdentityFixture()
	collection.Role = flow.RoleCollection
	verification := unittest.IdentityFixture()
	verification.Role = flow.RoleVerification
	execution := unittest.IdentityFixture()
	execution.Role = flow.RoleExecution

	allParitipants := append(participants, collection, verification, execution)

	genesis := run.GenerateRootBlock(allParitipants, run.GenerateRootSeal([]byte{}))

	nodes := make([]*Node, 0, len(participants))
	for _, identity := range participants {
		node := createNode(t, identity, participants, &genesis)
		nodes = append(nodes, node)
	}

	return nodes
}

func createNode(t *testing.T, identity *flow.Identity, participants flow.IdentityList, genesis *flow.Block) *Node {
	db, dbDir := unittest.TempBadgerDB(t)
	state, err := protocol.NewState(db)
	require.NoError(t, err)

	err = state.Mutate().Bootstrap(genesis)
	require.NoError(t, err)

	// find index
	index := len(participants)
	for i := 0; i < len(participants); i++ {
		if identity == participants[i] {
			index = i
			break
		}
	}
	require.NotEqual(t, index, len(participants), "can not find identity in participants")

	localID := identity.ID()

	// log with node index
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Int("index", index).Hex("local_id", localID[:]).Logger()
	notifier := notifications.NewLogConsumer(log)

	// initialize no-op metrics mock
	metrics := &module.Metrics{}
	metrics.On("HotStuffBusyDuration", mock.Anything, mock.Anything)
	metrics.On("HotStuffIdleDuration", mock.Anything, mock.Anything)

	// make local
	priv := helper.MakeBLSKey(t)
	local, err := local.New(identity, priv)
	require.NoError(t, err)
	fmt.Printf("local id: %v\n", local.NodeID())

	views := &Views{}

	// make network
	net := &Network{}

	headersDB := storage.NewHeaders(db)
	payloadsDB := storage.NewPayloads(db)
	blocksDB := storage.NewBlocks(db)

	guarantees, err := stdmap.NewGuarantees(100000)
	require.NoError(t, err)
	seals, err := stdmap.NewSeals(100000)
	require.NoError(t, err)

	// initialize the block builder
	chainID := "chain"
	build := builder.NewBuilder(db, guarantees, seals,
		builder.WithChainID(chainID),
	)

	signer := &Signer{identity.ID()}

	// initialize the pending blocks cache
	cache := buffer.NewPendingBlocks()

	rootHeader := &genesis.Header
	rootQC := &model.QuorumCertificate{
		View:      genesis.View,
		BlockID:   genesis.ID(),
		SignerIDs: nil, // TODO
		SigData:   nil,
	}
	selector := filter.Any

	final := &Finalizer{}

	communicator := &Communicator{}

	// initialize the block finalizer
	hot, err := consensus.NewParticipant(log, notifier, metrics, headersDB,
		views, state, local, build, final, signer, communicator, selector, rootHeader,
		rootQC)

	require.NoError(t, err)

	// initialize the compliance engine
	comp, err := compliance.New(log, net, local, state, headersDB, payloadsDB, nil, cache)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synchronization.New(log, net, local, state, blocksDB, comp)
	require.NoError(t, err)

	comp = comp.WithSynchronization(sync).WithConsensus(hot)

	node := &Node{
		db:         db,
		dbDir:      dbDir,
		id:         identity,
		compliance: comp,
		sync:       sync,
		hot:        hot,
		headers:    headersDB,
	}
	return node
}

func Test3Nodes(t *testing.T) {
	nodes := createNodes(t, 3)
	defer cleanupNodes(nodes)

	fmt.Printf("%v nodes created", len(nodes))
	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(n *Node) {
			<-n.compliance.Ready()
			<-n.hot.Wait()
			wg.Done()
		}(n)
	}
	wg.Wait()
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		n.db.Close()
		os.RemoveAll(n.dbDir)
	}
}
