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

type Finalizer struct{}

func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {
	return nil
}

// ===

// move to in memory network
type SubmitFunc func(uint8, interface{}, ...flow.Identifier) error

type Conduit struct {
	channelID uint8
	submit    SubmitFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return c.submit(c.channelID, event, targetIDs...)
}

type Network struct {
	engines  map[uint8]network.Engine
	conduits []*Conduit
}

func NewNetwork() *Network {
	return &Network{
		engines:  make(map[uint8]network.Engine),
		conduits: make([]*Conduit, 0),
	}
}

func (n *Network) Register(code uint8, engine network.Engine) (network.Conduit, error) {
	n.engines[code] = engine
	// the submit function needs the access to all the nodes,
	// so will be added later
	c := &Conduit{
		channelID: code,
	}
	n.conduits = append(n.conduits, c)
	return c, nil
}

func (n *Network) WithSubmit(submit SubmitFunc) *Network {
	for _, conduit := range n.conduits {
		conduit.submit = submit
	}
	return n
}

type Node struct {
	db         *badger.DB
	dbDir      string
	index      int
	id         *flow.Identity
	compliance *compliance.Engine
	sync       *synchronization.Engine
	hot        *hotstuff.EventLoop
	headers    *storage.Headers
	views      *storage.Views
	net        *Network
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

	// make network
	net := NewNetwork()

	headersDB := storage.NewHeaders(db)
	payloadsDB := storage.NewPayloads(db)
	blocksDB := storage.NewBlocks(db)
	viewsDB := storage.NewViews(db)

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

	// initialize the compliance engine
	comp, err := compliance.New(log, net, local, state, headersDB, payloadsDB, nil, cache)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synchronization.New(log, net, local, state, blocksDB, comp)
	require.NoError(t, err)

	// initialize the block finalizer
	hot, err := consensus.NewParticipant(log, notifier, metrics, headersDB,
		viewsDB, state, local, build, final, signer, comp, selector, rootHeader,
		rootQC)

	require.NoError(t, err)

	comp = comp.WithSynchronization(sync).WithConsensus(hot)

	node := &Node{
		db:         db,
		dbDir:      dbDir,
		index:      index,
		id:         identity,
		compliance: comp,
		sync:       sync,
		hot:        hot,
		headers:    headersDB,
		views:      viewsDB,
		net:        net,
	}
	return node
}

func blockNothing(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
	return true, 0
}

func Test3Nodes(t *testing.T) {
	nodes := createNodes(t, 1)
	defer cleanupNodes(nodes)

	connect(nodes, blockNothing)

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

type BlockOrDelayFunc func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration)

func connect(nodes []*Node, blockOrDelay BlockOrDelayFunc) {
	nodeDict := make(map[flow.Identifier]*Node, len(nodes))
	for _, n := range nodes {
		nodeDict[n.id.ID()] = n
	}

	for _, n := range nodes {
		{
			sender := n
			submit := func(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
				for _, targetID := range targetIDs {
					// find receiver
					receiver, found := nodeDict[targetID]
					if !found {
						continue
					}

					blocked, delay := blockOrDelay(channelID, event, sender, receiver)
					if blocked {
						continue
					}

					go receiveWithDelay(channelID, event, sender, receiver, delay)
				}

				return nil
			}
			n.net.WithSubmit(submit)
		}
	}
}

func receiveWithDelay(channelID uint8, event interface{}, sender, receiver *Node, delay time.Duration) {
	// delay the message sending
	if delay > 0 {
		time.Sleep(delay)
	}

	// find receiver engine
	receiverEngine := receiver.net.engines[channelID]

	// give it to receiver engine
	receiverEngine.Submit(sender.id.ID(), event)
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		n.db.Close()
		os.RemoveAll(n.dbDir)
	}
}
