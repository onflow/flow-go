package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/consensus"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network"
	networkmock "github.com/dapperlabs/flow-go/network/mock"
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

func (c *StopperConsumer) OnStartingTimeout(info *model.TimerInfo) {
	// fmt.Printf("start timeout for view %v, timeout ends %v\n", info.View, info.Duration)
	threshold := 30 * time.Second
	if info.Duration > threshold {
		panic(fmt.Sprintf("stop,%v", info.Duration))
	}
}

type Stopper struct {
	sync.Mutex
	running    map[flow.Identifier]struct{}
	nodes      []*Node
	stopping   bool
	stopAtView uint64
	stopped    chan struct{}
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
		stopped:    make(chan struct{}),
	}
}

func (s *Stopper) AddNode(n *Node) *StopperConsumer {
	s.Lock()
	defer s.Unlock()
	s.running[n.id.ID()] = struct{}{}
	s.nodes = append(s.nodes, n)
	return &StopperConsumer{
		onEnteringView: func(view uint64) {
			s.onEnteringView(n.id.ID(), view)
		},
	}
}

func (s *Stopper) onEnteringView(id flow.Identifier, view uint64) {
	s.Lock()
	defer s.Unlock()

	if view < s.stopAtView {
		return
	}

	// keep track of remaining running nodes
	delete(s.running, id)

	// if there is no running nodes, stop all
	if len(s.running) == 0 {
		fmt.Printf("stopall!\n")
		s.stopAll()
	}
}

func (s *Stopper) stopAll() {
	// has been stopped before
	if s.stopping {
		return
	}

	s.stopping = true

	// wait until all nodes has been shut down
	var wg sync.WaitGroup
	for i := 0; i < len(s.nodes); i++ {
		wg.Add(1)
		// stop compliance will also stop both hotstuff and synchronization engine
		go func(i int) {
			fmt.Printf("node %v shutting down\n", i)
			s.nodes[i].compliance.Done()
			fmt.Printf("node %v done\n", i)
			wg.Done()
		}(i)
	}
	wg.Wait()
	close(s.stopped)
	fmt.Printf("all nodes have been stopped\n")
}

type Node struct {
	db            *badger.DB
	dbDir         string
	index         int
	id            *flow.Identity
	compliance    *compliance.Engine
	sync          *synchronization.Engine
	hot           *hotstuff.EventLoop
	state         *protocol.State
	headers       *storage.Headers
	views         *storage.Views
	net           *Network
	syncblock     int
	blockproposal int
	blockvote     int
	syncreq       int
	syncresp      int
	rangereq      int
	batchreq      int
	batchresp     int
	sync.Mutex
}

func createNodes(t *testing.T, n int, stopAtView uint64) ([]*Node, *Stopper) {
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

	return nodes, stopper
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

	stopConsumer := stopper.AddNode(node)

	// log with node index
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(ioutil.Discard).Level(zerolog.DebugLevel).With().Timestamp().Int("index", index).Hex("local_id", localID[:]).Logger()
	// log := zerolog.New(os.Stderr).Level(zerolog.InfoLevel).With().Timestamp().Int("index", index).Hex("local_id", localID[:]).Logger()
	notifier := notifications.NewLogConsumer(log)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(stopConsumer)
	dis.AddConsumer(notifier)

	// initialize no-op metrics mock
	metrics := &module.Metrics{}
	metrics.On("HotStuffBusyDuration", mock.Anything, mock.Anything)
	metrics.On("HotStuffIdleDuration", mock.Anything, mock.Anything)
	metrics.On("HotStuffWaitDuration", mock.Anything, mock.Anything)

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
	build := builder.NewBuilder(db, guarantees, seals)

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
	selector := filter.HasRole(flow.RoleConsensus)

	// initialize the block finalizer
	final := finalizer.NewFinalizer(db, guarantees, seals)

	prov := &networkmock.Engine{}

	// initialize the compliance engine
	comp, err := compliance.New(log, net, local, state, headersDB, payloadsDB, prov, cache)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synchronization.New(log, net, local, state, blocksDB, comp)
	require.NoError(t, err)

	hotstuffTimeout := 200 * time.Millisecond

	// initialize the block finalizer
	hot, err := consensus.NewParticipant(log, dis, metrics, headersDB,
		viewsDB, state, local, build, final, signer, comp, selector, rootHeader,
		rootQC, consensus.WithTimeout(hotstuffTimeout))

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
		block, notBlock := true, false
		if _, ok := blackDict[sender.id.ID()]; ok {
			return block, 0
		}
		if _, ok := blackDict[receiver.id.ID()]; ok {
			return block, 0
		}
		return notBlock, 0
	}
}

func blockNodesForFirstNMessages(n int, blackList ...*Node) BlockOrDelayFunc {
	blackDict := make(map[flow.Identifier]*Node, len(blackList))
	for _, n := range blackList {
		blackDict[n.id.ID()] = n
	}

	sent, received := 0, 0

	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block, notBlock := true, false

		switch event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		default:
			return notBlock, 0
		}

		if _, ok := blackDict[sender.id.ID()]; ok {
			if sent >= n {
				return notBlock, 0
			}
			sent++
			fmt.Printf("sent: %v\n", sent)
			return block, 0
		}
		if _, ok := blackDict[receiver.id.ID()]; ok {
			if received >= n {
				return notBlock, 0
			}
			fmt.Printf("received: %v\n", received)
			received++
			return block, 0
		}
		return false, 0
	}
}

func blockProposals() BlockOrDelayFunc {
	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		switch event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		case *events.SyncedBlock:
		case *messages.SyncRequest:
		case *messages.SyncResponse:
		case *messages.RangeRequest:
		case *messages.BatchRequest:
		case *messages.BlockResponse:
		default:
			panic(fmt.Sprintf("wrong message from channel %v, type: %v", channelID, reflect.TypeOf(event)))
		}
		return false, 0
	}
}

func runNodes(nodes []*Node) {
	for _, n := range nodes {
		go func(n *Node) {
			<-n.compliance.Ready()
		}(n)
	}
}

func Test3Nodes(t *testing.T) {
	nodes, stopper := createNodes(t, 3, 100)

	connect(nodes, blockProposals())

	runNodes(nodes)

	<-stopper.stopped

	// verify all nodes arrive the same state
	for i := 0; i < len(nodes); i++ {
		headerN, err := nodes[i].state.Final().Head()
		require.NoError(t, err)
		require.Greater(t, headerN.View, uint64(90))
		printState(t, nodes, i)

	}
	cleanupNodes(nodes)
}

// with 5 nodes, and one node completely blocked, the other nodes can still reach consensus
func Test5Nodes(t *testing.T) {
	nodes, stopper := createNodes(t, 5, 100)

	connect(nodes, blockNodes(nodes[0]))

	runNodes(nodes)

	<-stopper.stopped

	header, err := nodes[0].state.Final().Head()
	require.NoError(t, err)

	// the first node was blocked, never finalize any block
	require.Equal(t, uint64(0), header.View)

	// verify all nodes arrive the same state
	for i := 0; i < len(nodes); i++ {
		printState(t, nodes, i)
	}
	for i := 1; i < len(nodes); i++ {
		n := nodes[i]
		headerN, err := n.state.Final().Head()
		require.NoError(t, err)
		require.Greater(t, headerN.View, uint64(90))
	}

	cleanupNodes(nodes)
}

func TestOneDelayed(t *testing.T) {
	nodes, stopper := createNodes(t, 5, 100)

	connect(nodes, blockNodesForFirstNMessages(100, nodes[0]))

	runNodes(nodes)

	<-stopper.stopped

	// verify all nodes arrive the same state
	for i := 0; i < len(nodes); i++ {
		printState(t, nodes, i)
	}
	for i := 1; i < len(nodes); i++ {
		n := nodes[i]
		headerN, err := n.state.Final().Head()
		require.NoError(t, err)
		require.Greater(t, headerN.View, uint64(90))
	}

	cleanupNodes(nodes)
}

func printState(t *testing.T, nodes []*Node, i int) {
	n := nodes[i]
	headerN, err := nodes[i].state.Final().Head()
	require.NoError(t, err)
	fmt.Printf("instance %v view:%v, height: %v,received syncblock:%v,proposal:%v,vote:%v,syncreq:%v,syncresp:%v,rangereq:%v,batchreq:%v,batchresp:%v\n",
		i, headerN.Height, headerN.View, n.syncblock, n.blockproposal, n.blockvote, n.syncreq, n.syncresp, n.rangereq, n.batchreq, n.batchresp)
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

	// statics
	receiver.Lock()
	switch event.(type) {
	case *events.SyncedBlock:
		receiver.syncblock++
	case *messages.BlockProposal:
		receiver.blockproposal++
	case *messages.BlockVote:
		receiver.blockvote++
	case *messages.SyncRequest:
		receiver.syncreq++
	case *messages.SyncResponse:
		receiver.syncresp++
	case *messages.RangeRequest:
		receiver.rangereq++
	case *messages.BatchRequest:
		receiver.batchreq++
	case *messages.BlockResponse:
		receiver.batchresp++
	default:
		panic("received message")
	}
	receiver.Unlock()

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
