package integration_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/network"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
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

type StopperConsumer struct {
	notifications.NoopConsumer
	onEnteringView func(view uint64)
}

func (c *StopperConsumer) OnEnteringView(view uint64) {
	c.onEnteringView(view)
}

type Stopper struct {
	sync.Mutex
	running    map[flow.Identifier]struct{}
	nodes      []*Node
	stopping   bool
	stopAtView uint64
}

// How to stop nodes?
// We can stop each node as soon as it enters a certain view. But the problem
// is if some fast nodes reaches a view earlier and gets stopped, it won't
// be available for other nodes to sync, and slow nodes will never be able
// to catch up.
// a better strategy is to wait until all nodes has entered a certain view,
// then stop them all.
func NewStopper(stopAtView uint64) *Stopper {
	return &Stopper{
		running:    make(map[flow.Identifier]struct{}),
		nodes:      make([]*Node, 0),
		stopping:   false,
		stopAtView: stopAtView,
	}
}

func (s *Stopper) AddNode(n *Node) *StopperConsumer {
	s.Lock()
	defer s.Unlock()
	s.running[n.id.ID()] = struct{}{}
	s.nodes = append(s.nodes, n)
	return &StopperConsumer{
		onEnteringView: func(view uint64) {
			s.OnEnteringView(n.id.ID(), view)
		},
	}
}

func (s *Stopper) OnEnteringView(id flow.Identifier, view uint64) {
	s.Lock()
	defer s.Unlock()

	if view < s.stopAtView {
		return
	}

	// keep track of remaining running nodes
	delete(s.running, id)

	// if there is no running nodes, stop all
	if len(s.running) == 0 {
		s.stopAll()
	}
}

func (s *Stopper) stopAll() {
	// has been stopped before
	if s.stopping {
		return
	}

	s.stopping = true

	var wg sync.WaitGroup
	for i := 0; i < len(s.nodes); i++ {
		wg.Add(1)
		// stop compliance will also stop both hotstuff and synchronization engine
		go func(i int) {
			fmt.Printf("===> %v closing instance to done \n", i)
			<-s.nodes[i].compliance.Done()
			wg.Done()
			fmt.Printf("===> %v instance is done\n", i)
		}(i)
	}
	wg.Wait()
}

func stopAll(nodes []*Node) {
	var wg sync.WaitGroup
	for i := 0; i < len(nodes); i++ {
		wg.Add(1)
		// stop compliance will also stop both hotstuff and synchronization engine
		go func(i int) {
			fmt.Printf("===> %v closing instance to done \n", i)
			<-nodes[i].compliance.Done()
			wg.Done()
			fmt.Printf("===> %v instance is done\n", i)
		}(i)
	}
	wg.Wait()
}

type Engine struct{}

func (*Engine) SubmitLocal(event interface{})                             {}
func (*Engine) Submit(originID flow.Identifier, event interface{})        {}
func (*Engine) ProcessLocal(event interface{}) error                      { return nil }
func (*Engine) Process(originID flow.Identifier, event interface{}) error { return nil }

type Node struct {
	db         *badger.DB
	dbDir      string
	index      int
	id         *flow.Identity
	compliance *compliance.Engine
	sync       *synchronization.Engine
	hot        *hotstuff.EventLoop
	state      *protocol.State
	headers    *storage.Headers
	views      *storage.Views
	net        *Network
}

func createNodes(t *testing.T, n int, stopAtView uint64) []*Node {
	// create n consensus nodes
	participants := make([]*flow.Identity, 0)
	for i := 0; i < n; i++ {
		identity := unittest.IdentityFixture()
		participants = append(participants, identity)
	}

	// create non-consensus nodes
	collection := unittest.IdentityFixture()
	collection.Role = flow.RoleCollection

	verification := unittest.IdentityFixture()
	verification.Role = flow.RoleVerification

	execution := unittest.IdentityFixture()
	execution.Role = flow.RoleExecution

	allParitipants := append(participants, collection, verification, execution)

	// add all identities to genesis block and
	// create and bootstrap consensus node with the genesis
	genesis := run.GenerateRootBlock(allParitipants, run.GenerateRootSeal([]byte{}))

	stopper := NewStopper(stopAtView)
	nodes := make([]*Node, 0, len(participants))
	for _, identity := range participants {
		node := createNode(t, identity, participants, &genesis, stopper)
		nodes = append(nodes, node)
	}

	return nodes
}

func stopNodes(nodes ...*Node) {
	for i := 0; i < len(nodes); i++ {

	}
}

func createNode(t *testing.T, identity *flow.Identity, participants flow.IdentityList, genesis *flow.Block, stopper *Stopper) *Node {
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

	node := &Node{
		db:    db,
		dbDir: dbDir,
		index: index,
		id:    identity,
	}

	// log with node index
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Int("index", index).Hex("local_id", localID[:]).Logger()

	// make local
	priv := helper.MakeBLSKey(t)
	local, err := local.New(identity, priv)
	require.NoError(t, err)

	// make network
	net := NewNetwork()

	headersDB := storage.NewHeaders(db)
	blocksDB := storage.NewBlocks(db)
	viewsDB := storage.NewViews(db)

	// initialize the compliance engine
	comp, err := compliance.New(log, net, local)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synchronization.New(log, net, local, state, blocksDB, comp)
	require.NoError(t, err)

	// initialize the block finalizer
	hot, err := consensus.NewParticipant(log)

	require.NoError(t, err)

	comp = comp.WithSynchronization(sync).WithConsensus(hot)

	node.compliance = comp
	node.sync = sync
	node.state = state
	node.hot = hot
	node.headers = headersDB
	node.views = viewsDB
	node.net = net

	return node
}

func blockNothing(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
	return false, 0
}

func blockNodes(blackList ...*Node) BlockOrDelayFunc {
	blackDict := make(map[flow.Identifier]*Node, len(blackList))
	for _, n := range blackList {
		blackDict[n.id.ID()] = n
	}
	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		if _, ok := blackDict[sender.id.ID()]; ok {
			return true, 0
		}
		if _, ok := blackDict[receiver.id.ID()]; ok {
			return true, 0
		}
		return false, 0
	}
}

func blockNodesForFirstNMessages(n int, blackList ...*Node) BlockOrDelayFunc {
	blackDict := make(map[flow.Identifier]*Node, len(blackList))
	for _, n := range blackList {
		blackDict[n.id.ID()] = n
	}

	sent, received := 0, 0

	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		if _, ok := blackDict[sender.id.ID()]; ok {
			if sent > n {
				return false, 0
			}
			sent++
			return true, 0
		}
		if _, ok := blackDict[receiver.id.ID()]; ok {
			if received > n {
				return false, 0
			}
			received++
			return true, 0
		}
		return false, 0
	}
}

func start(nodes []*Node) {
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

func Test3Nodes(t *testing.T) {
	nodes := createNodes(t, 3, 10)

	connect(nodes)

	go start(nodes)

	time.Sleep(3 * time.Second)

	stopAll(nodes)

	cleanupNodes(nodes)
}

type BlockOrDelayFunc func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration)

func connect(nodes []*Node) {
	nodeDict := make(map[flow.Identifier]*Node, len(nodes))
	for _, n := range nodes {
		nodeDict[n.id.ID()] = n
	}

	for _, n := range nodes {
		{
			submit := func(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
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
