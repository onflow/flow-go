package dkg

import (
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
)

// node is a test object that simulates a running instance of the DKG protocol
// where transitions from one phase to another are dictated by a timer.
type node struct {
	id         int
	controller *Controller

	// artifacts of DKG
	priv crypto.PrivateKey
	pub  crypto.PublicKey
	pubs []crypto.PublicKey
}

func newNode(id int, controller *Controller) *node {
	return &node{
		id:         id,
		controller: controller,
	}
}

func (n *node) run() error {
	go n.controller.Run()

	phaseDuration := 1 * time.Second

	err := n.delayedTask(phaseDuration, n.controller.EndPhase0)
	if err != nil {
		return err
	}

	err = n.delayedTask(phaseDuration, n.controller.EndPhase1)
	if err != nil {
		return err
	}

	err = n.delayedTask(
		phaseDuration,
		func() error {
			n.priv, n.pub, n.pubs, err = n.controller.End()
			return err
		})
	if err != nil {
		return err
	}

	return nil
}

func (n *node) delayedTask(delay time.Duration, task func() error) error {
	timer := time.After(delay)
	for {
		select {
		case <-timer:
			err := task()
			if err != nil {
				return err
			}
			return nil
		}
	}
}

// processor is an implementation of DKGProcessor that enables nodes to exchange
// private and public messages.
type processor struct {
	id       int
	channels []chan DKGMessage
}

func (proc *processor) PrivateSend(dest int, data []byte) {
	proc.channels[dest] <- DKGMessage{Orig: proc.id, Data: data}
}

func (proc *processor) Broadcast(data []byte) {
	for i := 0; i < len(proc.channels); i++ {
		if i == proc.id {
			continue
		}
		proc.channels[i] <- DKGMessage{Orig: proc.id, Data: data}
	}
}

func (proc *processor) Blacklist(node int) {}

func (proc *processor) FlagMisbehavior(node int, logData string) {}

func TestDKG(t *testing.T) {
	n := 5

	nodes := initNodes(t, n)

	// Start all nodes in parallel
	for _, n := range nodes {
		go func(node *node) {
			err := node.run()
			require.NoError(t, err)
		}(n)
	}

	// Wait until they are all shutdown
	timeout := time.After(10 * time.Second)
WAIT:
	for {
		select {
		case <-timeout:
			t.Fatal("TIMEOUT")
		default:
			done := true
			for _, node := range nodes {
				if node.controller.GetState() != Shutdown {
					done = false
					break
				}
			}
			if done {
				break WAIT
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Check that all nodes have agreed on the same set of public keys
	for i := 1; i < n; i++ {
		require.NotEmpty(t, nodes[i].priv)
		require.NotEmpty(t, nodes[i].pub)
		require.NotEmpty(t, nodes[i].pubs)
		require.Equal(t, nodes[0].pubs, nodes[i].pubs)
	}
}

func initNodes(t *testing.T, n int) []*node {
	// Create the channels through which the nodes will communicate
	channels := make([]chan DKGMessage, 0, n)
	for i := 0; i < n; i++ {
		channels = append(channels, make(chan DKGMessage, 5*n))
	}

	nodes := make([]*node, 0, n)

	// Setup
	for i := 0; i < n; i++ {
		seed := make([]byte, 20)
		rand.Read(seed)

		processor := &processor{
			id:       i,
			channels: channels,
		}

		dkg, err := crypto.NewJointFeldman(n, optimalThreshold(n), i, processor)
		require.NoError(t, err)

		controller := NewController(
			dkg,
			seed,
			channels[i],
			zerolog.New(os.Stderr).With().Int("id", i).Logger())

		node := newNode(i, controller)

		nodes = append(nodes, node)
	}

	return nodes
}

// optimal threshold (t) to allow the largest number of malicious nodes (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}
