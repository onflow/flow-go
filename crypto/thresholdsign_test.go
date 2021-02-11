// +build relic

package crypto

import (
	"crypto/rand"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThresholdSignature(t *testing.T) {
	t.Run("stateful_simple_keygen", testStatefulThresholdSignatureSimpleKeyGen)
	t.Run("stateful_signature_feldmanVSS_keygen", testStatefulThresholdSignatureFeldmanVSS)
	t.Run("stateful_signature_jointFeldman_keygen", testStatefulThresholdSignatureJointFeldman)
	t.Run("stateless_simple_keygen", testStatelessThresholdSignatureSimpleKeyGen)
	t.Run("stateless_signature_feldmanVSS_keygen", testStatelessThresholdSignatureFeldmanVSS)
}

const thresholdSignatureTag = "Threshold Signatures"

// simple single-threaded test of the stateful threshold signature using the simple key generation.
// The test generates keys for a threshold signatures scheme, uses the keys to sign shares,
// tests VerifyAndStageShare and CommitShare apis,
// and reconstruct the threshold signatures using (t+1) random shares.
func testStatefulThresholdSignatureSimpleKeyGen(t *testing.T) {
	n := 10
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		// generate threshold keys
		mrand.Seed(time.Now().UnixNano())
		seed := make([]byte, SeedMinLenDKG)
		_, err := mrand.Read(seed)
		require.NoError(t, err)
		skShares, pkShares, pkGroup, err := ThresholdSignKeyGen(n, threshold, seed)
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
		index := 0
		ts, err := NewThresholdSigner(n, threshold, index, kmac)
		require.NoError(t, err)
		err = ts.SetKeys(skShares[index], pkGroup, pkShares)
		require.NoError(t, err)
		ts.SetMessageToSign(messageToSign)
		// commit a non staged share
		err = ts.CommitShare()
		assert.Error(t, err)
		// create (t+1) signatures of the first randomly chosen signers
		for j := 0; j < threshold+1; j++ {
			i := signers[j]
			share, err := skShares[i].Sign(messageToSign, kmac)
			require.NoError(t, err)
			verif, err := ts.VerifyAndStageShare(i, share)
			assert.NoError(t, err)
			assert.True(t, verif, "signature should be valid")
			// check that threshold is not reached before the commit
			assert.False(t, ts.EnoughShares(), "threshold shouldn't be reached")
			if verif {
				err = ts.CommitShare()
				assert.NoError(t, err)
			}
		}
		// verify and commit when threshold is reached
		if threshold+1 < n {
			i := signers[threshold+1]
			share, err := skShares[i].Sign(messageToSign, kmac)
			require.NoError(t, err)
			verif, err := ts.VerifyAndStageShare(i, share)
			assert.NoError(t, err)
			assert.True(t, verif, "signature should be valid")
			if verif {
				err = ts.CommitShare()
				assert.NoError(t, err)
			}
		}
		// verify an existing share
		i := signers[0]
		share, err := skShares[i].Sign(messageToSign, kmac)
		require.NoError(t, err)
		verif, err := ts.VerifyAndStageShare(i, share)
		assert.NoError(t, err)
		assert.False(t, verif, "signature should be invalid")
		// commit a non staged share
		err = ts.CommitShare()
		assert.Error(t, err)

		// check that threshold is reached
		assert.True(t, ts.EnoughShares(), "threshold should be reached")
		// reconstruct the threshold signature
		_, err = ts.ThresholdSignature()
		require.NoError(t, err)
	}
}

// Testing Threshold Signature stateful api
// keys are generated using simple Feldman VSS
func testStatefulThresholdSignatureFeldmanVSS(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	// number of nodes to test
	n := 5
	lead := 0
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
func testStatefulThresholdSignatureJointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
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

// Testing Threshold Signature statless api
// keys are generated using simple Feldman VSS
func testStatelessThresholdSignatureFeldmanVSS(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	// number of nodes to test
	n := 5
	lead := 0
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
		go tsStatelessRunChan(&processors[i], &sync, t)
	}
	// synchronize the main thread to end TS
	sync.Wait()
}

var messageToSign = []byte{1, 2, 3}

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

		// if timeout, finalize DKG and sign the share
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
				proc.ts, err = NewThresholdSigner(n, optimalThreshold(n), proc.current, kmac)
				require.NoError(t, err)
				err = proc.ts.SetKeys(sk, groupPK, nodesPK)
				require.NoError(t, err)
				proc.ts.SetMessageToSign(messageToSign)
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
	sighShare, _ := proc.ts.SignShare()
	proc.protocol = tsType
	proc.Broadcast(sighShare)
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving TS from %d:", proc.current, newMsg.orig)
			verif, err := proc.ts.AddShare(
				newMsg.orig, newMsg.data)
			require.NoError(t, err)
			assert.True(t, verif,
				"the signature share sent from %d to %d is not correct", newMsg.orig,
				proc.current)
			if proc.ts.EnoughShares() {
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
	ownSignShare, _ := proc.keys.currentPrivateKey.Sign(messageToSign, kmac)
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
			verif, err := proc.keys.publicKeyShares[newMsg.orig].Verify(newMsg.data, messageToSign, kmac)
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
				verif, err = proc.keys.groupPublicKey.Verify(thresholdSignature, messageToSign, kmac)
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

// simple single-threaded test of threshold signature using the simple key generation.
// The test generates keys for a threshold signatures scheme, uses the keys to sign shares,
// and reconstruct the threshold signatures using (t+1) random shares.
func testStatelessThresholdSignatureSimpleKeyGen(t *testing.T) {
	n := 10
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		// generate threshold keys
		mrand.Seed(time.Now().UnixNano())
		seed := make([]byte, SeedMinLenDKG)
		_, err := mrand.Read(seed)
		require.NoError(t, err)
		skShares, pkShares, pkGroup, err := ThresholdSignKeyGen(n, threshold, seed)
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
			share, err := skShares[i].Sign(messageToSign, kmac)
			require.NoError(t, err)
			verif, err := pkShares[i].Verify(share, messageToSign, kmac)
			require.NoError(t, err)
			assert.True(t, verif, "signature share is not valid")
			if verif {
				signShares = append(signShares, share)
			}
		}
		// reconstruct and test the threshold signature
		thresholdSignature, err := ReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
		require.NoError(t, err)
		verif, err := pkGroup.Verify(thresholdSignature, messageToSign, kmac)
		require.NoError(t, err)
		assert.True(t, verif, "signature share is not valid")

		// check failure with a random redundant signer
		if threshold > 1 {
			randomDuplicate := mrand.Intn(int(threshold)) + 1 // 1 <= duplicate <= threshold
			signers[randomDuplicate] = signers[0]
			thresholdSignature, err = ReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
			assert.Error(t, err)
		}
	}
}

func BenchmarkSimpleKeyGen(b *testing.B) {
	n := 60
	seed := make([]byte, SeedMinLenDKG)
	rand.Read(seed)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = ThresholdSignKeyGen(n, optimalThreshold(n), seed)
	}
	b.StopTimer()
}
