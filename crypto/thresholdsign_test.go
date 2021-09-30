// +build relic

package crypto

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThresholdSignature(t *testing.T) {
	// stateless API
	//t.Run("centralized_stateless_keygen", testCentralizedStatelessAPI)
	// stateful API
	t.Run("centralized_stateful_keygen", testCentralizedStatefulAPI)
	//t.Run("distributed_stateful_feldmanVSS_keygen", testDistributedStatefulAPI_FeldmanVSS)
	//t.Run("distributed_stateful_jointFeldman_keygen", testDistributedStatefulAPI_JointFeldman) // Flow Random beacon case
}

const thresholdSignatureTag = "random tag"

var thresholdSignatureMessage = []byte("random message")

// simple centralized test of the stateful threshold signature using the simple key generation.
// The test generates keys for a threshold signatures scheme, uses the keys to sign shares,
// tests VerifyAndStageShare and CommitShare apis,
// and reconstruct the threshold signatures using (t+1) random shares.
func testCentralizedStatefulAPI(t *testing.T) {
	n := 10
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		// generate threshold keys
		mrand.Seed(time.Now().UnixNano())
		seed := make([]byte, SeedMinLenDKG)
		_, err := mrand.Read(seed)
		require.NoError(t, err)
		skShares, pkShares, pkGroup, err := ThresholdKeyGen(n, threshold, seed)
		require.NoError(t, err)
		// signature hasher
		kmac := NewBLSKMAC(thresholdSignatureTag)
		// generate signature shares
		signers := make([]int, 0, n)
		// fill the signers list and shuffle it
		for i := 0; i < n; i++ {
			signers = append(signers, i)
		}
		mrand.Shuffle(n, func(i, j int) {
			signers[i], signers[j] = signers[j], signers[i]
		})
		// create the stateful threshold signer
		index := mrand.Intn(n)
		ts, err := NewThresholdSigner(pkGroup, pkShares, threshold, index, skShares[index], thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		// check EnoughShares
		enough := ts.EnoughShares()
		assert.False(t, enough)
		var wg sync.WaitGroup
		// create (t) signatures of the first randomly chosen signers
		// ( 1 signature short of the threshold)
		for j := 0; j < threshold; j++ {
			wg.Add(1)
			// test thread safety
			go func(j int) {
				defer wg.Done()
				i := signers[j]
				share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
				require.NoError(t, err)
				// VerifyShare
				verif, err := ts.VerifyShare(i, share)
				assert.NoError(t, err)
				assert.True(t, verif, "signature should be valid")
				// check HasSignature is false
				ok := ts.HasShare(i)
				assert.False(t, ok)
				// TrustedAdd
				enough, err := ts.TrustedAdd(i, share)
				assert.NoError(t, err)
				assert.False(t, enough)
				// check HasSignature is true
				ok = ts.HasShare(i)
				assert.True(t, verif)
				// check EnoughSignature
				assert.False(t, ts.EnoughShares(), "threshold shouldn't be reached")
			}(j)
		}
		wg.Wait()
		// add the last required signature to get (t+1) shares
		i := signers[threshold]
		share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		verif, enough, err := ts.VerifyAndAdd(i, share)
		assert.NoError(t, err)
		assert.True(t, verif)
		assert.True(t, enough)
		// check EnoughSignature
		assert.True(t, ts.EnoughShares())

		// add a share when threshold is reached
		if threshold+1 < n {
			i := signers[threshold+1]
			share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			// Trusted Add
			enough, err := ts.TrustedAdd(i, share)
			assert.NoError(t, err)
			assert.True(t, enough)
			// VerifyAndAdd
			verif, enough, err := ts.VerifyAndAdd(i, share)
			assert.NoError(t, err)
			assert.True(t, verif)
			assert.True(t, enough)
		}

		// Add an existing share
		i = signers[0]
		share, err = skShares[i].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		// VerifyAndAdd
		verif, enough, err = ts.VerifyAndAdd(i, share)
		assert.Error(t, err)
		assert.False(t, IsInvalidInputsError(err))
		assert.False(t, verif)
		assert.False(t, enough)
		// TrustedAdd
		enough, err = ts.TrustedAdd(i, share)
		assert.Error(t, err)
		assert.False(t, IsInvalidInputsError(err))
		assert.False(t, enough)

		// reconstruct the threshold signature
		thresholdsignature, err := ts.ThresholdSignature()
		require.NoError(t, err)
		// VerifyThresholdSignature
		verif, err = ts.VerifyThresholdSignature(thresholdsignature)
		require.NoError(t, err)
		assert.True(t, verif)
	}
}

// Testing Threshold Signature stateful api
// keys are generated using simple Feldman VSS
func testDistributedStatefulAPI_FeldmanVSS(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	gt = t
	// number of nodes to test
	n := 5
	lead := mrand.Intn(n) // random leader
	var sync sync.WaitGroup
	chans := make([]chan *message, n)
	processors := make([]testDKGProcessor, 0, n)

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current:  current,
			chans:    chans,
			protocol: dkgType,
		})
		// create DKG in all nodes
		var err error
		processors[current].dkg, err = NewFeldmanVSS(n, optimalThreshold(n),
			current, &processors[current], lead)
		require.NoError(t, err)
	}

	// create the node (buffered) communication channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 2*n)
	}
	// start DKG in all nodes
	seed := make([]byte, SeedMinLenDKG)
	read, err := rand.Read(seed)
	require.Equal(t, read, SeedMinLenDKG)
	require.NoError(t, err)
	sync.Add(n)
	for current := 0; current < n; current++ {
		err := processors[current].dkg.Start(seed)
		require.NoError(t, err)
		go tsDkgRunChan(&processors[current], &sync, t, 2)
	}

	// synchronize the main thread to end DKG
	sync.Wait()
	for i := 1; i < n; i++ {
		assert.True(t, processors[i].pk.Equals(processors[0].pk), "2 group public keys are mismatching")
	}

	// Start TS
	log.Info("TS starts")
	sync.Add(n)
	for i := 0; i < n; i++ {
		go tsRunChan(&processors[i], &sync, t)
	}
	// synchronize the main thread to end TS
	sync.Wait()
}

// Testing Threshold Signature stateful api
// keys are generated using Joint-Feldman
func testDistributedStatefulAPI_JointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	gt = t
	// number of nodes to test
	n := 5
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		var sync sync.WaitGroup
		chans := make([]chan *message, n)
		processors := make([]testDKGProcessor, 0, n)

		// create n processors for all nodes
		for current := 0; current < n; current++ {
			processors = append(processors, testDKGProcessor{
				current:  current,
				chans:    chans,
				protocol: dkgType,
			})
			// create DKG in all nodes
			var err error
			processors[current].dkg, err = NewJointFeldman(n,
				optimalThreshold(n), current, &processors[current])
			require.NoError(t, err)
		}

		// create the node (buffered) communication channels
		for i := 0; i < n; i++ {
			chans[i] = make(chan *message, 2*n)
		}
		// start DKG in all nodes but the leader
		seed := make([]byte, SeedMinLenDKG)
		read, err := rand.Read(seed)
		require.Equal(t, read, SeedMinLenDKG)
		require.NoError(t, err)
		sync.Add(n)
		for current := 0; current < n; current++ {
			err := processors[current].dkg.Start(seed)
			require.NoError(t, err)
			go tsDkgRunChan(&processors[current], &sync, t, 0)
		}

		// sync the 2 timeouts at all nodes and start the next phase
		for phase := 1; phase <= 2; phase++ {
			sync.Wait()
			sync.Add(n)
			for current := 0; current < n; current++ {
				go tsDkgRunChan(&processors[current], &sync, t, phase)
			}
		}

		// synchronize the main thread to end DKG
		sync.Wait()
		for i := 1; i < n; i++ {
			assert.True(t, processors[i].pk.Equals(processors[0].pk),
				"2 group public keys are mismatching")
		}

		// Start TS
		log.Info("TS starts")
		sync.Add(n)
		for current := 0; current < n; current++ {
			go tsRunChan(&processors[current], &sync, t)
		}
		// synchronize the main thread to end TS
		sync.Wait()
	}
}

// This is a testing function
// It simulates processing incoming messages by a node during DKG
// It assumes proc.dkg is already running
func tsDkgRunChan(proc *testDKGProcessor,
	sync *sync.WaitGroup, t *testing.T, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving DKG from %d:", proc.current, newMsg.orig)
			if newMsg.channel == private {
				err := proc.dkg.HandlePrivateMsg(newMsg.orig, newMsg.data)
				require.Nil(t, err)
			} else {
				err := proc.dkg.HandleBroadcastMsg(newMsg.orig, newMsg.data)
				require.Nil(t, err)
			}

		// if timeout, finalize DKG and create the threshold signer
		case <-time.After(200 * time.Millisecond):
			switch phase {
			case 0:
				log.Infof("%d shares phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				require.NoError(t, err)
			case 1:
				log.Infof("%d complaints phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				require.NoError(t, err)
			case 2:
				log.Infof("%d dkg ended \n", proc.current)
				sk, groupPK, nodesPK, err := proc.dkg.End()
				require.NotNil(t, sk)
				require.NotNil(t, groupPK)
				require.NotNil(t, nodesPK)
				require.Nil(t, err, "End dkg failed: %v\n", err)
				proc.pk = groupPK
				n := proc.dkg.Size()
				kmac := NewBLSKMAC(thresholdSignatureTag)
				proc.ts, err = NewThresholdSigner(groupPK, nodesPK, optimalThreshold(n), proc.current, sk, thresholdSignatureMessage, kmac)
				require.NoError(t, err)
				// needed to test the statless api
				proc.keys = &statelessKeys{sk, groupPK, nodesPK}
			}
			sync.Done()
			return
		}
	}
}

// This is a testing function using the stateful api
// It simulates processing incoming messages by a node during TS
func tsRunChan(proc *testDKGProcessor, sync *sync.WaitGroup, t *testing.T) {
	// Sign a share and broadcast it
	sigShare, err := proc.ts.SignShare()
	proc.protocol = tsType
	if err != nil { // not using require.Nil for now
		panic(fmt.Sprintf("%d couldn't sign", proc.current))
	}
	proc.Broadcast(sigShare)
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving TS from %d:", proc.current, newMsg.orig)
			verif, enough, err := proc.ts.VerifyAndAdd(
				newMsg.orig, newMsg.data)
			require.NoError(t, err)
			assert.True(t, verif,
				"the signature share sent from %d to %d is not correct", newMsg.orig,
				proc.current)
			log.Info(enough)
			if enough {
				assert.Equal(t, enough, proc.ts.EnoughShares())
				thresholdSignature, err := proc.ts.ThresholdSignature()
				require.NoError(t, err)
				verif, err = proc.ts.VerifyThresholdSignature(thresholdSignature)
				require.NoError(t, err)
				assert.True(t, verif, "the threshold signature is not correct")
				if verif {
					log.Infof("%d reconstructed a valid signature: %d\n", proc.current,
						thresholdSignature)
				}
			}

		// if timeout, finalize TS
		case <-time.After(time.Second):
			sync.Done()
			return
		}
	}
}

// This stucture holds the keys and is needed for the stateless test
type statelessKeys struct {
	// the current node private key (a DKG output)
	currentPrivateKey PrivateKey
	// the group public key (a DKG output)
	groupPublicKey PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []PublicKey
}

// This is a testing function using the stateless api
// It simulates processing incoming messages by a node during TS
func tsStatelessRunChan(proc *testDKGProcessor, sync *sync.WaitGroup, t *testing.T) {
	n := proc.dkg.Size()
	// Sign a share and broadcast it
	kmac := NewBLSKMAC(thresholdSignatureTag)
	ownSignShare, _ := proc.keys.currentPrivateKey.Sign(thresholdSignatureMessage, kmac)
	// the local valid signature shares
	signShares := make([]Signature, 0, n)
	signers := make([]int, 0, n)
	// add the node own share
	signShares = append(signShares, ownSignShare)
	signers = append(signers, proc.current)
	proc.protocol = tsType
	proc.Broadcast(ownSignShare)
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving TS from %d:", proc.current, newMsg.orig)
			verif, err := proc.keys.publicKeyShares[newMsg.orig].Verify(newMsg.data, thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			assert.True(t, verif,
				"the signature share sent from %d to %d is not correct", newMsg.orig,
				proc.current)
			// append the received signature share
			if verif {
				// check the signer is new
				isSeen := true
				for _, i := range signers {
					if i == newMsg.orig {
						isSeen = false
					}
				}
				if isSeen {
					signShares = append(signShares, newMsg.data)
					signers = append(signers, newMsg.orig)
				}
			}
			threshReached, err := EnoughShares(optimalThreshold(n), len(signShares))
			assert.NoError(t, err)
			if threshReached {
				// Reconstruct the threshold signature
				thresholdSignature, err := ReconstructThresholdSignature(n, optimalThreshold(n), signShares, signers)
				assert.NoError(t, err)
				verif, err = proc.keys.groupPublicKey.Verify(thresholdSignature, thresholdSignatureMessage, kmac)
				require.NoError(t, err)
				assert.True(t, verif, "the threshold signature is not correct")
				if verif {
					log.Infof("%d reconstructed a valid signature: %d\n", proc.current,
						thresholdSignature)
				}
			}

		// if timeout, finalize TS
		case <-time.After(time.Second):
			sync.Done()
			return
		}
	}
}

// simple centralized test of threshold signature using the simple key generation.
// The test generates keys for a threshold signatures scheme, uses the keys to sign shares,
// and reconstruct the threshold signatures using (t+1) random shares.
func testCentralizedStatelessAPI(t *testing.T) {
	n := 10
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		// generate threshold keys
		mrand.Seed(time.Now().UnixNano())
		seed := make([]byte, SeedMinLenDKG)
		_, err := mrand.Read(seed)
		require.NoError(t, err)
		skShares, pkShares, pkGroup, err := ThresholdKeyGen(n, threshold, seed)
		require.NoError(t, err)
		// signature hasher
		kmac := NewBLSKMAC(thresholdSignatureTag)
		// generate signature shares
		signShares := make([]Signature, 0, n)
		signers := make([]int, 0, n)
		// fill the signers list and shuffle it
		for i := 0; i < n; i++ {
			signers = append(signers, i)
		}
		mrand.Shuffle(n, func(i, j int) {
			signers[i], signers[j] = signers[j], signers[i]
		})
		// create (t+1) signatures of the first randomly chosen signers
		for j := 0; j < threshold+1; j++ {
			i := signers[j]
			share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			verif, err := pkShares[i].Verify(share, thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			assert.True(t, verif, "signature share is not valid")
			if verif {
				signShares = append(signShares, share)
			}
		}
		// reconstruct and test the threshold signature
		thresholdSignature, err := ReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
		require.NoError(t, err)
		verif, err := pkGroup.Verify(thresholdSignature, thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		assert.True(t, verif, "signature share is not valid")

		// check failure with a random redundant signer
		if threshold > 1 {
			randomDuplicate := mrand.Intn(int(threshold)) + 1 // 1 <= duplicate <= threshold
			signers[randomDuplicate] = signers[0]
			thresholdSignature, err = ReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
			assert.Error(t, err)
			assert.IsType(t, expectedError, err)
		}
	}
}

func BenchmarkSimpleKeyGen(b *testing.B) {
	n := 60
	seed := make([]byte, SeedMinLenDKG)
	rand.Read(seed)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = ThresholdKeyGen(n, optimalThreshold(n), seed)
	}
	b.StopTimer()
}
