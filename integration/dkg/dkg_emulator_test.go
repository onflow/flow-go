package dkg

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

func TestWithEmulator(t *testing.T) {
	suite.Run(t, new(DKGSuite))
}

func (s *DKGSuite) runTest(goodNodes int, emulatorProblems bool) {

	nodes := s.nodes[:goodNodes]

	// The EpochSetup event is received at view 100.  The current epoch is
	// configured with phase transitions at views 150, 200, and 250. In between
	// phase transitions, the controller calls the DKG smart-contract every 10
	// views.
	//
	// VIEWS
	// setup      : 100
	// polling    : 110 120 130 140 150
	// Phase1Final: 150
	// polling    : 160 170 180 190 200
	// Phase2Final: 200
	// polling    : 210 220 230 240 250
	// Phase3Final: 250
	// final

	// we arbitrarily use 999 as the current epoch counter
	currentCounter := uint64(999)

	currentEpochSetup := flow.EpochSetup{
		Counter:            currentCounter,
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 250,
		FinalView:          300,
		Participants:       s.netIDs,
		RandomSource:       []byte("random bytes for seed"),
	}

	// create the EpochSetup that will trigger the next DKG run with all the
	// desired parameters
	nextEpochSetup := flow.EpochSetup{
		Counter:      currentCounter + 1,
		Participants: s.netIDs,
		RandomSource: []byte("random bytes for seed"),
	}

	firstBlock := &flow.Header{View: 100}

	for _, node := range nodes {
		node.setEpochs(s.T(), currentEpochSetup, nextEpochSetup, firstBlock)
	}

	for _, n := range nodes {
		n.Ready()
	}

	// trigger the EpochSetupPhaseStarted event for all nodes, effectively
	// starting the next DKG run
	for _, n := range nodes {
		n.ProtocolEvents.EpochSetupPhaseStarted(currentCounter, firstBlock)
	}

	// submit a lot of dummy transactions to force the creation of blocks and
	// views
	view := 0
	for view < 300 {
		time.Sleep(100 * time.Millisecond)

		// if we are testing situations where the DKG smart-contract is not
		// reachable, disable the DKG client for intervals of 10 views
		if emulatorProblems {
			for _, node := range nodes {
				if view%20 >= 10 {
					node.dkgContractClient.Disable()
				} else {
					node.dkgContractClient.Enable()
				}
			}
		}

		// deliver private messages
		s.hub.DeliverAll()

		// submit a tx to force the emulator to create and finalize a block
		block, err := s.sendDummyTx()

		if err == nil {
			for _, node := range nodes {
				node.ProtocolEvents.BlockFinalized(block.Header)
			}
			view = int(block.Header.View)
		}
	}

	// before ending the test and awaiting successful completion, ensure we leave
	// the dkg client in an enabled state
	for _, node := range nodes {
		node.dkgContractClient.Enable()
	}

	for _, n := range nodes {
		n.Done()
	}

	// DKG is completed if one value was proposed by a majority of nodes
	completed := s.isDKGCompleted()
	assert.True(s.T(), completed)

	// the result is an array of public keys where the first item is the group
	// public key
	res := s.getResult()

	assert.Equal(s.T(), len(s.nodes)+1, len(res))
	pubKeys := make([]crypto.PublicKey, 0, len(res))
	for _, r := range res {
		pkBytes, err := hex.DecodeString(r)
		assert.NoError(s.T(), err)
		pk, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pkBytes)
		assert.NoError(s.T(), err)
		pubKeys = append(pubKeys, pk)
	}

	groupPubKeyBytes, err := hex.DecodeString(res[0])
	assert.NoError(s.T(), err)
	groupPubKey, err := crypto.DecodePublicKey(crypto.BLSBLS12381, groupPubKeyBytes)
	assert.NoError(s.T(), err)

	// create and test a threshold signature with the keys computed by dkg
	sigData := []byte("message to be signed")
	signers := make([]*signature.ThresholdProvider, 0, len(nodes))
	signatures := []crypto.Signature{}
	indices := []uint{}
	for i, n := range nodes {
		priv, err := n.keyStorage.RetrieveMyBeaconPrivateKey(nextEpochSetup.Counter)
		require.NoError(s.T(), err)

		signer := signature.NewThresholdProvider("TAG", priv.RandomBeaconPrivKey.PrivateKey)
		signers = append(signers, signer)

		signature, err := signer.Sign(sigData)
		require.NoError(s.T(), err)

		signatures = append(signatures, signature)
		indices = append(indices, uint(i))

		ok, err := signer.Verify(sigData, signature, pubKeys[1+i])
		require.NoError(s.T(), err)
		assert.True(s.T(), ok, fmt.Sprintf("signature %d share doesn't verify under the public key share", i+1))
	}

	// shuffle the signatures and indices before constructing the group
	// signature (since it only uses the first half signatures)
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	rand.Shuffle(len(signatures), func(i, j int) {
		signatures[i], signatures[j] = signatures[j], signatures[i]
		indices[i], indices[j] = indices[j], indices[i]
	})

	// NOTE: Reconstruction doesn't require a tag or local, but is only accessible
	// through the broader Provider API, hence the empty arguments.
	thresholdSigner := signature.NewThresholdProvider("", nil)
	groupSignature, err := thresholdSigner.Reconstruct(uint(len(s.nodes)), signatures, indices)
	require.NoError(s.T(), err)

	ok, err := signers[0].Verify(sigData, groupSignature, groupPubKey)
	require.NoError(s.T(), err)
	assert.True(s.T(), ok, "failed to verify threshold signature")
}

// TestHappyPath checks that DKG works when all nodes are good
func (s *DKGSuite) TestHappyPath() {
	s.runTest(numberOfNodes, false)
}

// TestNodesDown checks that DKG still works with the maximum number of bad
// nodes.
func (s *DKGSuite) TestNodesDown() {
	minHonestNodes := numberOfNodes - signature.RandomBeaconThreshold(numberOfNodes)
	s.runTest(minHonestNodes, false)
}

// TestEmulatorProblems checks that DKG is resilient to transient problems
// between the node and the DKG smart-contract ( this covers connection issues
// between consensus node and access node, as well as connection issues between
// access node and execution node, or the execution node being down).
func (s *DKGSuite) TestEmulatorProblems() {
	s.T().Skip("flaky test - quarantined")
	s.runTest(numberOfNodes, true)
}
