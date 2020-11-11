package dkg

import (
	"crypto/rand"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
)

// node is a test object that simulates a running instance of the DKG protocol
// where transitions from one phase to another are dictated by a timer.
type node struct {
	id            int
	controller    *Controller
	phaseDuration time.Duration
}

func newNode(id int, controller *Controller, phaseDuration time.Duration) *node {
	return &node{
		id:            id,
		controller:    controller,
		phaseDuration: phaseDuration,
	}
}

func (n *node) run() error {
	// start the DKG controller
	go func() {
		_ = n.controller.Run()
	}()

	// Wait and trigger transition from Phase 0 to Phase 1
	err := n.delayedTask(n.phaseDuration, n.controller.EndPhase0)
	if err != nil {
		return err
	}

	// Wait and trigger transition from Phase 1 to Phase 2
	err = n.delayedTask(n.phaseDuration, n.controller.EndPhase1)
	if err != nil {
		return err
	}

	// Wait and retrieve DKG results
	err = n.delayedTask(
		n.phaseDuration,
		func() error {
			err := n.controller.End()
			return err
		})
	if err != nil {
		return err
	}

	return nil
}

func (n *node) delayedTask(delay time.Duration, task func() error) error {
	timer := time.After(delay)
	<-timer
	return task()
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

type testCase struct {
	totalNodes    int
	phaseDuration time.Duration
}

// TestDKGNormal tests the controller in optimal conditions, when all nodes are
// working correctly.
func TestDKGNormal(t *testing.T) {
	// define different test cases with varying number of nodes, and phase
	// durations
	testCases := []testCase{
		testCase{totalNodes: 5, phaseDuration: time.Second},
		testCase{totalNodes: 10, phaseDuration: time.Second},
		testCase{totalNodes: 15, phaseDuration: 3 * time.Second},
		testCase{totalNodes: 20, phaseDuration: 3 * time.Second},
	}

	// run each test case
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d nodes", tc.totalNodes), func(t *testing.T) {
			testDKG(t, tc.totalNodes, tc.totalNodes, tc.phaseDuration)
		})
	}
}

// TestDKGThreshold tests that the controller results in a successful DKG as
// long as the minimum threshold for non-byzantine nodes is satisfied.
func TestDKGThreshold(t *testing.T) {
	// define different test cases with varying number of nodes, and phase
	// durations
	testCases := []testCase{
		testCase{totalNodes: 5, phaseDuration: time.Second},
		testCase{totalNodes: 10, phaseDuration: time.Second},
		testCase{totalNodes: 15, phaseDuration: 3 * time.Second},
		testCase{totalNodes: 20, phaseDuration: 3 * time.Second},
	}

	// run each test case
	for _, tc := range testCases {
		// gn is the minimum number of good nodes required for the DKG protocol
		// to go well
		gn := tc.totalNodes - optimalThreshold(tc.totalNodes)

		t.Run(fmt.Sprintf("%d/%d nodes", gn, tc.totalNodes), func(t *testing.T) {
			testDKG(t, tc.totalNodes, gn, tc.phaseDuration)
		})
	}
}

// ATTENTION: This test seems to indicate that nodes end up with a different set
// of public keys when the phase duration is short.
func TestDKGShortDuration(t *testing.T) {
	testDKG(t, 20, 20, 2*time.Second)
}

func testDKG(t *testing.T, totalNodes int, goodNodes int, phaseDuration time.Duration) {
	nodes := initNodes(t, totalNodes, phaseDuration)
	gnodes := nodes[:goodNodes]

	// Start all the good nodes in parallel
	for _, n := range gnodes {
		go func(node *node) {
			err := node.run()
			require.NoError(t, err)
		}(n)
	}

	// Wait until they are all shutdown
	wait(t, gnodes, 5*phaseDuration)

	// Check that all nodes have agreed on the same set of public keys
	checkArtifacts(t, gnodes, totalNodes)
}

// Initialise nodes and communication channels.
func initNodes(t *testing.T, n int, phaseDuration time.Duration) []*node {
	// Create the channels through which the nodes will communicate
	channels := make([]chan DKGMessage, 0, n)
	for i := 0; i < n; i++ {
		channels = append(channels, make(chan DKGMessage, 5*n*n))
	}

	nodes := make([]*node, 0, n)

	// Setup
	for i := 0; i < n; i++ {
		seed := make([]byte, 20)
		_, _ = rand.Read(seed)

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

		node := newNode(i, controller, phaseDuration)

		nodes = append(nodes, node)
	}

	return nodes
}

// Wait for all the nodes to reach the SHUTDOWN state, or timeout.
func wait(t *testing.T, nodes []*node, timeout time.Duration) {

	timer := time.After(timeout)

	for {
		select {
		case <-timer:
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
				return
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// Check that all nodes have produced the same set of public keys
func checkArtifacts(t *testing.T, nodes []*node, totalNodes int) {
	for i := 1; i < len(nodes); i++ {
		require.NotEmpty(t, nodes[i].controller.privateShare)
		require.NotEmpty(t, nodes[i].controller.publicShare)
		require.Len(t, nodes[i].controller.publicKeys, totalNodes)

		for j := 0; j < totalNodes; j++ {
			if !reflect.DeepEqual(nodes[0].controller.publicKeys[j], nodes[i].controller.publicKeys[j]) {
				t.Fatalf("pubs[%d] differ: %s, %s", j,
					nodes[0].controller.publicKeys[j].String(),
					nodes[i].controller.publicKeys[j].String())
			}
		}

	}
}

// optimal threshold (t) to allow the largest number of malicious nodes (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}
