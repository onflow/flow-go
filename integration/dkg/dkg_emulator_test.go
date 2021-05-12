package dkg

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestWithEmulator(t *testing.T) {
	suite.Run(t, new(DKGSuite))
}

func (s *DKGSuite) TestHappyPath() {

	// The EpochSetup event is received at view 100. The phase transitions are
	// at views 150, 200, and 250. In between phase transitions, the controller
	// calls the DKG smart-contract every 10 views.
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

	// create the EpochSetup that will trigger the next DKG run with all the
	// desired parameters
	epochSetup := flow.EpochSetup{
		Counter:            999,
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 250,
		FinalView:          300,
		Participants:       s.netIDs,
		RandomSource:       []byte("random bytes for seed"),
	}
	firstBlock := &flow.Header{View: 100}

	for _, node := range s.nodes {
		node.setEpochSetup(s.T(), epochSetup, firstBlock)
	}

	for _, n := range s.nodes {
		n.Ready()
	}

	// trigger the EpochSetupPhaseStarted event for all nodes, effectively
	// starting the next DKG run
	for _, n := range s.nodes {
		n.ProtocolEvents.EpochSetupPhaseStarted(epochSetup.Counter, firstBlock)
	}

	// submit a lot of dummy transactions to force the creation of blocks and
	// views
	view := 0
	for view < 300 {
		time.Sleep(200 * time.Millisecond)

		// deliver private messages
		s.hub.DeliverAll()

		// submit a tx to force the emulator to create and finalize a block
		block, err := s.sendDummyTx()

		if err == nil {
			for _, node := range s.nodes {
				node.ProtocolEvents.BlockFinalized(block.Header)
			}
			view = int(block.Header.View)
		}
	}

	for _, n := range s.nodes {
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
	signers := make([]*signature.ThresholdProvider, 0, len(s.nodes))
	signatures := []crypto.Signature{}
	indices := []uint{}
	for i, n := range s.nodes {
		priv, err := n.keyStorage.RetrieveMyDKGPrivateInfo(epochSetup.Counter)
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

	groupSignature, err := signature.CombineThresholdShares(uint(len(s.nodes)), signatures, indices)
	require.NoError(s.T(), err)

	ok, err := signers[0].Verify(sigData, groupSignature, groupPubKey)
	require.NoError(s.T(), err)
	assert.True(s.T(), ok, "failed to verify threshold signature")
}
