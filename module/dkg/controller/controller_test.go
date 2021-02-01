package controller

import (
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	msg "github.com/onflow/flow-go/model/messages"
)

// node is a test object that simulates a running instance of the DKG protocol
// where transitions from one phase to another are dictated by a timer.
type node struct {
	id             int
	controller     *Controller
	phase0Duration time.Duration
	phase1Duration time.Duration
	phase2Duration time.Duration
}

func newNode(id int, controller *Controller,
	phase0Duration time.Duration,
	phase1Duration time.Duration,
	phase2Duration time.Duration) *node {

	return &node{
		id:             id,
		controller:     controller,
		phase0Duration: phase0Duration,
		phase1Duration: phase1Duration,
		phase2Duration: phase2Duration,
	}
}

func (n *node) run() error {

	// runErrCh is used to receive potential errors from the async DKG run
	// routine
	runErrCh := make(chan error)

	// start the DKG controller
	go func() {
		runErrCh <- n.controller.Run()
	}()

	// timers to control phase transitions
	var phase0Timer <-chan time.Time
	var phase1Timer <-chan time.Time
	var phase2Timer <-chan time.Time

	phase0Timer = time.After(n.phase0Duration)

	for {
		select {
		case err := <-runErrCh:
			// received an error from the async run routine
			return fmt.Errorf("Async Run error: %w", err)
		case <-phase0Timer:
			// end of phase 0
			err := n.controller.EndPhase0()
			if err != nil {
				return fmt.Errorf("Error transitioning to Phase 1: %w", err)
			}
			phase1Timer = time.After(n.phase1Duration)
		case <-phase1Timer:
			// end of phase 1
			err := n.controller.EndPhase1()
			if err != nil {
				return fmt.Errorf("Error transitioning to Phase 2: %w", err)
			}
			phase2Timer = time.After(n.phase2Duration)
		case <-phase2Timer:
			// end of phase 2
			err := n.controller.End()
			if err != nil {
				return fmt.Errorf("Error ending DKG: %w", err)
			}
			return nil
		}
	}
}

// processor is an implementation of DKGProcessor that enables nodes to exchange
// private and public messages.
type processor struct {
	id       int
	channels []chan msg.DKGMessage
	logger   zerolog.Logger
}

func (proc *processor) PrivateSend(dest int, data []byte) {
	proc.channels[dest] <- msg.NewDKGMessage(
		proc.id,
		data,
		0, 0) // epoch and phase are not relevant at the controller level
}

// ATTENTION: Normally the processor requires Broadcast to provide guaranteed
// delivery (either all nodes receive the message or none of them receive it).
// Here we are just assuming that with a long enough duration for phases 1 and
// 2, all nodes are guaranteed to see everyone's messages. So it is important
// to set timeouts carefully in the tests.
func (proc *processor) Broadcast(data []byte) {
	for i := 0; i < len(proc.channels); i++ {
		if i == proc.id {
			continue
		}
		// epoch and phase are not relevant at the controller level
		proc.channels[i] <- msg.NewDKGMessage(proc.id, data, 0, 0)
	}
}

func (proc *processor) Disqualify(node int, log string) {
	proc.logger.Debug().Msgf("node %d disqualified node %d: %s", proc.id, node, log)
}

func (proc *processor) FlagMisbehavior(node int, logData string) {
	proc.logger.Debug().Msgf("node %d flagged node %d: %s", proc.id, node, logData)
}

type testCase struct {
	totalNodes     int
	phase0Duration time.Duration
	phase1Duration time.Duration
	phase2Duration time.Duration
}

// TestDKGHappyPath tests the controller in optimal conditions, when all nodes are
// working correctly.
func TestDKGHappyPath(t *testing.T) {
	// Define different test cases with varying number of nodes, and phase
	// durations. Since these are all happy path cases, there are no messages
	// sent during phase 1 and 2, all messaging is done in phase 0. So we can
	// can set shorter durations for phase 1 and 2..
	testCases := []testCase{
		testCase{totalNodes: 5, phase0Duration: time.Second, phase1Duration: 100 * time.Millisecond, phase2Duration: 100 * time.Millisecond},
		testCase{totalNodes: 10, phase0Duration: time.Second, phase1Duration: 100 * time.Millisecond, phase2Duration: 100 * time.Millisecond},
		testCase{totalNodes: 15, phase0Duration: 5 * time.Second, phase1Duration: 2 * time.Second, phase2Duration: 2 * time.Second},
	}

	// run each test case
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d nodes", tc.totalNodes), func(t *testing.T) {
			testDKG(t, tc.totalNodes, tc.totalNodes, tc.phase0Duration, tc.phase1Duration, tc.phase2Duration)
		})
	}
}

// TestDKGThreshold tests that the controller results in a successful DKG as
// long as the minimum threshold for non-byzantine nodes is satisfied.
func TestDKGThreshold(t *testing.T) {
	// define different test cases with varying number of nodes, and phase
	// durations
	testCases := []testCase{
		testCase{totalNodes: 5, phase0Duration: time.Second, phase1Duration: time.Second, phase2Duration: time.Second},
		testCase{totalNodes: 10, phase0Duration: time.Second, phase1Duration: time.Second, phase2Duration: time.Second},
		testCase{totalNodes: 15, phase0Duration: 5 * time.Second, phase1Duration: 2 * time.Second, phase2Duration: 2 * time.Second},
	}

	// run each test case
	for _, tc := range testCases {
		// gn is the minimum number of good nodes required for the DKG protocol
		// to go well
		gn := tc.totalNodes - optimalThreshold(tc.totalNodes)

		t.Run(fmt.Sprintf("%d/%d nodes", gn, tc.totalNodes), func(t *testing.T) {
			testDKG(t, tc.totalNodes, gn, tc.phase0Duration, tc.phase1Duration, tc.phase2Duration)
		})
	}
}

func testDKG(t *testing.T, totalNodes int, goodNodes int, phase0Duration, phase1Duration, phase2Duration time.Duration) {
	nodes := initNodes(t, totalNodes, phase0Duration, phase1Duration, phase2Duration)
	gnodes := nodes[:goodNodes]

	// Start all the good nodes in parallel
	for _, n := range gnodes {
		go func(node *node) {
			err := node.run()
			require.NoError(t, err)
		}(n)
	}

	// Wait until they are all shutdown
	wait(t, gnodes, 5*phase0Duration)

	// Check that all nodes have agreed on the same set of public keys
	checkArtifacts(t, gnodes, totalNodes)
}

// Initialise nodes and communication channels.
func initNodes(t *testing.T, n int, phase0Duration, phase1Duration, phase2Duration time.Duration) []*node {
	// Create the channels through which the nodes will communicate
	channels := make([]chan msg.DKGMessage, 0, n)
	for i := 0; i < n; i++ {
		channels = append(channels, make(chan msg.DKGMessage, 5*n*n))
	}

	nodes := make([]*node, 0, n)

	// Setup
	for i := 0; i < n; i++ {
		seed := make([]byte, 20)
		_, _ = rand.Read(seed)

		logger := zerolog.New(os.Stderr).With().Int("id", i).Logger()

		processor := &processor{
			id:       i,
			channels: channels,
			logger:   logger,
		}

		dkg, err := crypto.NewJointFeldman(n, optimalThreshold(n), i, processor)
		require.NoError(t, err)

		controller := NewController(
			dkg,
			seed,
			channels[i],
			logger)

		node := newNode(i, controller, phase0Duration, phase1Duration, phase2Duration)

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
		require.NotEmpty(t, nodes[i].controller.groupPublicKey)

		require.True(t, nodes[0].controller.groupPublicKey.Equals(nodes[i].controller.groupPublicKey),
			"node %d has a different groupPubKey than node 0: %s %s",
			i,
			nodes[i].controller.groupPublicKey,
			nodes[0].controller.groupPublicKey)

		require.Len(t, nodes[i].controller.publicKeys, totalNodes)

		for j := 0; j < totalNodes; j++ {
			if !nodes[0].controller.publicKeys[j].Equals(nodes[i].controller.publicKeys[j]) {
				t.Fatalf("node %d has a different pubs[%d] than node 0: %s, %s",
					i,
					j,
					nodes[0].controller.publicKeys[j],
					nodes[i].controller.publicKeys[j])
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
