package dkg

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// node is a test object that simulates a running instance of the DKG protocol
// where transitions from one phase to another are dictated by a timer.
type node struct {
	id             int
	controller     *Controller
	phase1Duration time.Duration
	phase2Duration time.Duration
	phase3Duration time.Duration
}

func newNode(id int, controller *Controller,
	phase1Duration time.Duration,
	phase2Duration time.Duration,
	phase3Duration time.Duration) *node {

	return &node{
		id:             id,
		controller:     controller,
		phase1Duration: phase1Duration,
		phase2Duration: phase2Duration,
		phase3Duration: phase3Duration,
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
	var phase1Timer <-chan time.Time
	var phase2Timer <-chan time.Time
	var phase3Timer <-chan time.Time

	phase1Timer = time.After(n.phase1Duration)

	for {
		select {
		case err := <-runErrCh:
			// received an error from the async run routine
			return fmt.Errorf("Async Run error: %w", err)
		case <-phase1Timer:
			err := n.controller.EndPhase1()
			if err != nil {
				return fmt.Errorf("Error transitioning to Phase 2: %w", err)
			}
			phase2Timer = time.After(n.phase2Duration)
		case <-phase2Timer:
			err := n.controller.EndPhase2()
			if err != nil {
				return fmt.Errorf("Error transitioning to Phase 3: %w", err)
			}
			phase3Timer = time.After(n.phase3Duration)
		case <-phase3Timer:
			err := n.controller.End()
			if err != nil {
				return fmt.Errorf("Error ending DKG: %w", err)
			}
			return nil
		}
	}
}

// broker is a test implementation of DKGBroker that enables nodes to exchange
// private and public messages through a shared set of channels.
type broker struct {
	id                int
	privateChannels   []chan msg.DKGMessage
	broadcastChannels []chan msg.DKGMessage
	logger            zerolog.Logger
	dkgInstanceID     string
}

// PrivateSend implements the crypto.DKGProcessor interface.
func (b *broker) PrivateSend(dest int, data []byte) {
	b.privateChannels[dest] <- msg.NewDKGMessage(b.id, data, b.dkgInstanceID)
}

// Broadcast implements the crypto.DKGProcessor interface.
//
// ATTENTION: Normally the processor requires Broadcast to provide guaranteed
// delivery (either all nodes receive the message or none of them receive it).
// Here we are just assuming that with a long enough duration for phases 2 and
// 3, all nodes are guaranteed to see everyone's messages. So it is important
// to set timeouts carefully in the tests.
func (b *broker) Broadcast(data []byte) {
	for i := 0; i < len(b.broadcastChannels); i++ {
		if i == b.id {
			continue
		}
		// epoch and phase are not relevant at the controller level
		b.broadcastChannels[i] <- msg.NewDKGMessage(b.id, data, b.dkgInstanceID)
	}
}

// Disqualify implements the crypto.DKGProcessor interface.
func (b *broker) Disqualify(node int, log string) {
	b.logger.Error().Msgf("node %d disqualified node %d: %s", b.id, node, log)
}

// FlagMisbehavior implements the crypto.DKGProcessor interface.
func (b *broker) FlagMisbehavior(node int, logData string) {
	b.logger.Error().Msgf("node %d flagged node %d: %s", b.id, node, logData)
}

// GetIndex implements the DKGBroker interface.
func (b *broker) GetIndex() int {
	return int(b.id)
}

// GetPrivateMsgCh implements the DKGBroker interface.
func (b *broker) GetPrivateMsgCh() <-chan msg.DKGMessage {
	return b.privateChannels[b.id]
}

// GetBroadcastMsgCh implements the DKGBroker interface.
func (b *broker) GetBroadcastMsgCh() <-chan msg.DKGMessage {
	return b.broadcastChannels[b.id]
}

// Poll implements the DKGBroker interface.
func (b *broker) Poll(referenceBlock flow.Identifier) error { return nil }

// SubmitResult implements the DKGBroker interface.
func (b *broker) SubmitResult(crypto.PublicKey, []crypto.PublicKey) error { return nil }

// Shutdown implements the DKGBroker interface.
func (b *broker) Shutdown() {}

type testCase struct {
	totalNodes     int
	phase1Duration time.Duration
	phase2Duration time.Duration
	phase3Duration time.Duration
}

// TestDKGHappyPath tests the controller in optimal conditions, when all nodes
// are working correctly.
func TestDKGHappyPath(t *testing.T) {
	// Define different test cases with varying number of nodes, and phase
	// durations. Since these are all happy path cases, there are no messages
	// sent during phases 2 and 3; all messaging is done in phase 1. So we can
	// can set shorter durations for phases 2 and 3.
	testCases := []testCase{
		{totalNodes: 5, phase1Duration: 1 * time.Second, phase2Duration: 10 * time.Millisecond, phase3Duration: 10 * time.Millisecond},
		{totalNodes: 10, phase1Duration: 2 * time.Second, phase2Duration: 50 * time.Millisecond, phase3Duration: 50 * time.Millisecond},
		{totalNodes: 15, phase1Duration: 5 * time.Second, phase2Duration: 100 * time.Millisecond, phase3Duration: 100 * time.Millisecond},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d nodes", tc.totalNodes), func(t *testing.T) {
			testDKG(t, tc.totalNodes, tc.totalNodes, tc.phase1Duration, tc.phase2Duration, tc.phase3Duration)
		})
	}
}

// TestDKGThreshold tests that the controller results in a successful DKG as
// long as the minimum threshold for non-byzantine nodes is satisfied.
func TestDKGThreshold(t *testing.T) {
	// define different test cases with varying number of nodes, and phase
	// durations
	testCases := []testCase{
		{totalNodes: 5, phase1Duration: 1 * time.Second, phase2Duration: 100 * time.Millisecond, phase3Duration: 100 * time.Millisecond},
		{totalNodes: 10, phase1Duration: 2 * time.Second, phase2Duration: 500 * time.Millisecond, phase3Duration: 500 * time.Millisecond},
		{totalNodes: 15, phase1Duration: 5 * time.Second, phase2Duration: time.Second, phase3Duration: time.Second},
	}

	for _, tc := range testCases {
		// gn is the minimum number of good nodes required for the DKG protocol
		// to go well
		gn := tc.totalNodes - signature.RandomBeaconThreshold(tc.totalNodes)
		t.Run(fmt.Sprintf("%d/%d nodes", gn, tc.totalNodes), func(t *testing.T) {
			testDKG(t, tc.totalNodes, gn, tc.phase1Duration, tc.phase2Duration, tc.phase3Duration)
		})
	}
}

func testDKG(t *testing.T, totalNodes int, goodNodes int, phase1Duration, phase2Duration, phase3Duration time.Duration) {
	nodes := initNodes(t, totalNodes, phase1Duration, phase2Duration, phase3Duration)
	gnodes := nodes[:goodNodes]

	// Start all the good nodes in parallel
	for _, n := range gnodes {
		go func(node *node) {
			err := node.run()
			require.NoError(t, err)
		}(n)
	}

	// Wait until they are all shutdown
	wait(t, gnodes, 5*phase1Duration)

	// Check that all nodes have agreed on the same set of public keys
	checkArtifacts(t, gnodes, totalNodes)
}

// Initialise nodes and communication channels.
func initNodes(t *testing.T, n int, phase1Duration, phase2Duration, phase3Duration time.Duration) []*node {
	// Create the channels through which the nodes will communicate
	privateChannels := make([]chan msg.DKGMessage, 0, n)
	broadcastChannels := make([]chan msg.DKGMessage, 0, n)
	for i := 0; i < n; i++ {
		privateChannels = append(privateChannels, make(chan msg.DKGMessage, 5*n*n))
		broadcastChannels = append(broadcastChannels, make(chan msg.DKGMessage, 5*n*n))
	}

	nodes := make([]*node, 0, n)

	// Setup
	for i := 0; i < n; i++ {
		logger := zerolog.New(os.Stderr).With().Int("id", i).Logger()

		broker := &broker{
			id:                i,
			privateChannels:   privateChannels,
			broadcastChannels: broadcastChannels,
			logger:            logger,
		}

		seed := unittest.SeedFixture(20)

		dkg, err := crypto.NewJointFeldman(n, signature.RandomBeaconThreshold(n), i, broker)
		require.NoError(t, err)

		// create a config with no delays for tests
		config := ControllerConfig{
			BaseStartDelay:                 0,
			BaseHandleFirstBroadcastDelay:  0,
			HandleSubsequentBroadcastDelay: 0,
		}

		controller := NewController(
			logger,
			"dkg_test",
			dkg,
			seed,
			broker,
			config,
		)
		require.NoError(t, err)

		node := newNode(i, controller, phase1Duration, phase2Duration, phase3Duration)
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
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// Check that all nodes have produced the same set of public keys
func checkArtifacts(t *testing.T, nodes []*node, totalNodes int) {
	_, refGroupPublicKey, refPublicKeys := nodes[0].controller.GetArtifacts()

	for i := 1; i < len(nodes); i++ {
		privateShare, groupPublicKey, publicKeys := nodes[i].controller.GetArtifacts()

		require.NotEmpty(t, privateShare)
		require.NotEmpty(t, groupPublicKey)

		require.True(t, refGroupPublicKey.Equals(groupPublicKey),
			"node %d has a different groupPubKey than node 0: %s %s",
			i,
			groupPublicKey,
			refGroupPublicKey)

		require.Len(t, publicKeys, totalNodes)

		for j := 0; j < totalNodes; j++ {
			if !refPublicKeys[j].Equals(publicKeys[j]) {
				t.Fatalf("node %d has a different pubs[%d] than node 0: %s, %s",
					i,
					j,
					refPublicKeys[j],
					publicKeys[j])
			}
		}
	}
}

func TestDelay(t *testing.T) {

	t.Run("should return 0 delay for <=0 inputs", func(t *testing.T) {
		delay := computePreprocessingDelay(0, 100)
		assert.Equal(t, delay, time.Duration(0))
		delay = computePreprocessingDelay(time.Hour, 0)
		assert.Equal(t, delay, time.Duration(0))
		delay = computePreprocessingDelay(time.Millisecond, -1)
		assert.Equal(t, delay, time.Duration(0))
		delay = computePreprocessingDelay(-time.Millisecond, 100)
		assert.Equal(t, delay, time.Duration(0))
	})

	// NOTE: this is a probabilistic test. It will (extremely infrequently) fail.
	t.Run("should return different values for same inputs", func(t *testing.T) {
		d1 := computePreprocessingDelay(time.Hour, 100)
		d2 := computePreprocessingDelay(time.Hour, 100)
		assert.NotEqual(t, d1, d2)
	})

	t.Run("should return values in expected range", func(t *testing.T) {
		baseDelay := time.Second
		dkgSize := 100
		minDelay := time.Duration(0)
		// m=b*n^2
		expectedMaxDelay := time.Duration(int64(baseDelay) * int64(dkgSize) * int64(dkgSize))

		maxDelay := computePreprocessingDelayMax(baseDelay, dkgSize)
		assert.Equal(t, expectedMaxDelay, maxDelay)

		delay := computePreprocessingDelay(baseDelay, dkgSize)
		assert.LessOrEqual(t, minDelay, delay)
		assert.GreaterOrEqual(t, expectedMaxDelay, delay)
	})

	t.Run("should return values in expected range for defaults", func(t *testing.T) {
		baseDelay := DefaultBaseHandleFirstBroadcastDelay
		dkgSize := 150
		minDelay := time.Duration(0)
		// m=b*n^2
		expectedMaxDelay := time.Duration(int64(baseDelay) * int64(dkgSize) * int64(dkgSize))

		maxDelay := computePreprocessingDelayMax(baseDelay, dkgSize)
		assert.Equal(t, expectedMaxDelay, maxDelay)

		delay := computePreprocessingDelay(baseDelay, dkgSize)
		assert.LessOrEqual(t, minDelay, delay)
		assert.GreaterOrEqual(t, expectedMaxDelay, delay)
	})
}
