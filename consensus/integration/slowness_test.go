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

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/consensus/compliance"
	"github.com/dapperlabs/flow-go/model/flow"
)

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

type Node struct {
	db         *badger.DB
	dbDir      string
	index      int
	id         *flow.Identity
	compliance *compliance.Engine
	sync       *synchronization.Engine
	hot        *hotstuff.EventLoop
}

func createNodes(t *testing.T, n int, stopAtView uint64) []*Node {
	nodes := make([]*Node, 0)
	for i := 0; i < n; i++ {
		node := createNode(t, i)
		nodes = append(nodes, node)
	}

	return nodes
}

func createNode(t *testing.T, index int) *Node {
	// log with node index
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Int("index", index).Logger()
	// initialize the compliance engine
	comp, err := compliance.New(log)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synchronization.New(log)
	require.NoError(t, err)

	comp = comp.WithSynchronization(sync)

	node := &Node{
		compliance: comp,
		sync:       sync,
	}

	return node
}

func start(nodes []*Node) {
	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(n *Node) {
			<-n.compliance.Ready()
			wg.Done()
		}(n)
	}
	wg.Wait()
}

func TestStartStop(t *testing.T) {
	nodes := createNodes(t, 2, 10)

	go start(nodes)

	time.Sleep(3 * time.Second)

	stopAll(nodes)
}
