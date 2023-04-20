//go:build relic
// +build relic

package crypto

import (
	crand "crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBLSThresholdSignature(t *testing.T) {
	// stateless API
	t.Run("centralized_stateless_keygen", testCentralizedStatelessAPI)
	// stateful API
	t.Run("centralized_stateful_keygen", testCentralizedStatefulAPI)
	t.Run("distributed_stateful_feldmanVSS_keygen", testDistributedStatefulAPI_FeldmanVSS)
	t.Run("distributed_stateful_jointFeldman_keygen", testDistributedStatefulAPI_JointFeldman) // Flow Random beacon case
}

const thresholdSignatureTag = "random tag"

var thresholdSignatureMessage = []byte("random message")

// centralized test of the stateful threshold signature using the threshold key generation.
func testCentralizedStatefulAPI(t *testing.T) {
	n := 10
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		// generate threshold keys
		rand := getPRG(t)
		seed := make([]byte, SeedMinLenDKG)
		_, err := rand.Read(seed)
		require.NoError(t, err)
		skShares, pkShares, pkGroup, err := BLSThresholdKeyGen(n, threshold, seed)
		require.NoError(t, err)
		// generate signature shares
		signers := make([]int, 0, n)
		// hasher
		kmac := NewExpandMsgXOFKMAC128(thresholdSignatureTag)
		// fill the signers list and shuffle it
		for i := 0; i < n; i++ {
			signers = append(signers, i)
		}
		rand.Shuffle(n, func(i, j int) {
			signers[i], signers[j] = signers[j], signers[i]
		})

		t.Run("happy path", func(t *testing.T) {
			// create the stateful threshold signer
			ts, err := NewBLSThresholdSignatureInspector(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
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
					ok, err := ts.HasShare(i)
					assert.NoError(t, err)
					assert.False(t, ok)
					// TrustedAdd
					enough, err := ts.TrustedAdd(i, share)
					assert.NoError(t, err)
					assert.False(t, enough)
					// check HasShare is true
					ok, err = ts.HasShare(i)
					assert.NoError(t, err)
					assert.True(t, ok)
					// check EnoughSignature
					assert.False(t, ts.EnoughShares(), "threshold shouldn't be reached")
					// check ThresholdSignature
					sig, err := ts.ThresholdSignature()
					assert.Error(t, err)
					assert.True(t, IsNotEnoughSharesError(err))
					assert.Nil(t, sig)
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
			// reconstruct the threshold signature
			thresholdsignature, err := ts.ThresholdSignature()
			require.NoError(t, err)
			// VerifyThresholdSignature
			verif, err = ts.VerifyThresholdSignature(thresholdsignature)
			require.NoError(t, err)
			assert.True(t, verif)
		})

		t.Run("duplicate signer", func(t *testing.T) {
			// create the stateful threshold signer
			ts, err := NewBLSThresholdSignatureInspector(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
			require.NoError(t, err)

			// Create a share and add it
			i := rand.Intn(n)
			share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			enough, err := ts.TrustedAdd(i, share)
			assert.NoError(t, err)
			assert.False(t, enough)

			// Add an existing share

			// VerifyAndAdd
			verif, enough, err := ts.VerifyAndAdd(i, share)
			assert.Error(t, err)
			assert.True(t, IsDuplicatedSignerError(err))
			assert.False(t, verif)
			assert.False(t, enough)
			// TrustedAdd
			enough, err = ts.TrustedAdd(i, share)
			assert.Error(t, err)
			assert.True(t, IsDuplicatedSignerError(err))
			assert.False(t, enough)
		})

		t.Run("Invalid index", func(t *testing.T) {
			// create the stateful threshold signer
			ts, err := NewBLSThresholdSignatureInspector(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
			require.NoError(t, err)

			share, err := skShares[0].Sign(thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			// invalid index
			invalidIndex := len(pkShares) + 1
			// VerifyShare
			verif, err := ts.VerifyShare(invalidIndex, share)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.False(t, verif)
			// TrustedAdd
			enough, err := ts.TrustedAdd(invalidIndex, share)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.False(t, enough)
			// VerifyAndAdd
			verif, enough, err = ts.VerifyAndAdd(invalidIndex, share)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.False(t, verif)
			assert.False(t, enough)
			// HasShare
			verif, err = ts.HasShare(invalidIndex)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.False(t, verif)
		})

		t.Run("invalid signature", func(t *testing.T) {
			index := signers[0]
			ts, err := NewBLSThresholdSignatureInspector(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
			require.NoError(t, err)
			share, err := skShares[index].Sign(thresholdSignatureMessage, kmac)
			require.NoError(t, err)

			// alter signature - invalid serialization
			tmp := share[0]
			share[0] = invalidBLSSignatureHeader
			// VerifyShare
			verif, err := ts.VerifyShare(index, share)
			assert.NoError(t, err)
			assert.False(t, verif)
			// VerifyAndAdd
			verif, enough, err := ts.VerifyAndAdd(index, share)
			assert.NoError(t, err)
			assert.False(t, verif)
			assert.False(t, enough)
			// check share was not added
			verif, err = ts.HasShare(index)
			assert.NoError(t, err)
			assert.False(t, verif)
			// restore share
			share[0] = tmp

			// valid curve point but invalid signature
			otherIndex := (index + 1) % n // otherIndex is different than index
			// VerifyShare
			verif, err = ts.VerifyShare(otherIndex, share)
			assert.NoError(t, err)
			assert.False(t, verif)
			// VerifyAndAdd
			verif, enough, err = ts.VerifyAndAdd(otherIndex, share)
			assert.NoError(t, err)
			assert.False(t, verif)
			assert.False(t, enough)
			// check share was not added
			verif, err = ts.HasShare(otherIndex)
			assert.NoError(t, err)
			assert.False(t, verif)

			// trust add one invalid signature and check ThresholdSignature
			tmp = share[0]
			share[0] = invalidBLSSignatureHeader      // alter the share
			enough, err = ts.TrustedAdd(index, share) // invalid share
			assert.NoError(t, err)
			assert.False(t, enough)
			for i := 1; i < threshold+1; i++ { // valid shares
				index := signers[i]
				valid, err := skShares[index].Sign(thresholdSignatureMessage, kmac)
				require.NoError(t, err)
				enough, err = ts.TrustedAdd(index, valid)
				assert.NoError(t, err)
				if i < threshold {
					assert.False(t, enough)
				} else {
					assert.True(t, enough)
				}
			}
			sig, err := ts.ThresholdSignature()
			assert.Error(t, err)
			assert.True(t, IsInvalidSignatureError(err))
			assert.Nil(t, sig)
			share[0] = tmp // restore the share
		})

		t.Run("constructor errors", func(t *testing.T) {
			// invalid keys size
			index := rand.Intn(n)
			pkSharesInvalid := make([]PublicKey, ThresholdSignMaxSize+1)
			tsFollower, err := NewBLSThresholdSignatureInspector(pkGroup, pkSharesInvalid, threshold, thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, tsFollower)
			// non BLS key share
			seed := make([]byte, KeyGenSeedMinLen)
			_, err = rand.Read(seed)
			require.NoError(t, err)
			skEcdsa, err := GeneratePrivateKey(ECDSAP256, seed)
			require.NoError(t, err)
			tmp := pkShares[0]
			pkShares[0] = skEcdsa.PublicKey()
			tsFollower, err = NewBLSThresholdSignatureInspector(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsNotBLSKeyError(err))
			assert.Nil(t, tsFollower)
			pkShares[0] = tmp // restore valid keys
			// non BLS group key
			tsFollower, err = NewBLSThresholdSignatureInspector(skEcdsa.PublicKey(), pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsNotBLSKeyError(err))
			assert.Nil(t, tsFollower)
			// non BLS private key
			tsParticipant, err := NewBLSThresholdSignatureParticipant(pkGroup, pkShares, threshold, index, skEcdsa, thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsNotBLSKeyError(err))
			assert.Nil(t, tsParticipant)
			// invalid current index
			tsParticipant, err = NewBLSThresholdSignatureParticipant(pkGroup, pkShares, threshold, len(pkShares)+1, skShares[index], thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, tsParticipant)
			// invalid threshold
			tsFollower, err = NewBLSThresholdSignatureInspector(pkGroup, pkShares, len(pkShares)+1, thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, tsFollower)
			// inconsistent private and public key
			indexSwap := (index + 1) % n // indexSwap is different than index
			pkShares[index], pkShares[indexSwap] = pkShares[indexSwap], pkShares[index]
			tsParticipant, err = NewBLSThresholdSignatureParticipant(pkGroup, pkShares, len(pkShares)+1, index, skShares[index], thresholdSignatureMessage, thresholdSignatureTag)
			assert.Error(t, err)
			assert.True(t, IsInvalidInputsError(err))
			assert.Nil(t, tsParticipant)
			pkShares[index], pkShares[indexSwap] = pkShares[indexSwap], pkShares[index] // restore keys
		})
	}
}

// Distributed Threshold Signature stateful api test
// keys are generated using simple Feldman VSS
func testDistributedStatefulAPI_FeldmanVSS(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	gt = t
	rand := getPRG(t)
	// number of participants to test
	n := 5
	lead := rand.Intn(n) // random
	var sync sync.WaitGroup
	chans := make([]chan *message, n)
	processors := make([]testDKGProcessor, 0, n)

	// create n processors for all participants
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current:  current,
			chans:    chans,
			protocol: dkgType,
		})
		// create DKG in all participants
		var err error
		processors[current].dkg, err = NewFeldmanVSS(n, optimalThreshold(n),
			current, &processors[current], lead)
		require.NoError(t, err)
	}

	// create the participant (buffered) communication channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 2*n)
	}
	// start DKG in all participants
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

// Distributed  Threshold Signature stateful api test
// keys are generated using Joint-Feldman
func testDistributedStatefulAPI_JointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	gt = t
	rand := getPRG(t)
	// number of participants to test
	n := 5
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		var sync sync.WaitGroup
		chans := make([]chan *message, n)
		processors := make([]testDKGProcessor, 0, n)

		// create n processors for all participants
		for current := 0; current < n; current++ {
			processors = append(processors, testDKGProcessor{
				current:  current,
				chans:    chans,
				protocol: dkgType,
			})
			// create DKG in all participants
			var err error
			processors[current].dkg, err = NewJointFeldman(n,
				optimalThreshold(n), current, &processors[current])
			require.NoError(t, err)
		}

		// create the participant (buffered) communication channels
		for i := 0; i < n; i++ {
			chans[i] = make(chan *message, 2*n)
		}
		// start DKG in all participants but the
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

		// sync the 2 timeouts at all participants and start the next phase
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
// It simulates processing incoming messages by a participant during DKG
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
				proc.ts, err = NewBLSThresholdSignatureParticipant(groupPK, nodesPK, optimalThreshold(n), proc.current, sk, thresholdSignatureMessage, thresholdSignatureTag)
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
// It simulates processing incoming messages by a participant during TS
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
	// the current participant private key (a DKG output)
	myPrivateKey PrivateKey
	// the group public key (a DKG output)
	groupPublicKey PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []PublicKey
}

// Centralized test of threshold signature protocol using the threshold key generation.
func testCentralizedStatelessAPI(t *testing.T) {
	rand := getPRG(t)
	n := 10
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		// generate threshold keys
		seed := make([]byte, SeedMinLenDKG)
		_, err := rand.Read(seed)
		require.NoError(t, err)
		skShares, pkShares, pkGroup, err := BLSThresholdKeyGen(n, threshold, seed)
		require.NoError(t, err)
		// signature hasher
		kmac := NewExpandMsgXOFKMAC128(thresholdSignatureTag)
		// generate signature shares
		signShares := make([]Signature, 0, n)
		signers := make([]int, 0, n)
		// fill the signers list and shuffle it
		for i := 0; i < n; i++ {
			signers = append(signers, i)
		}
		rand.Shuffle(n, func(i, j int) {
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
		thresholdSignature, err := BLSReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
		require.NoError(t, err)
		verif, err := pkGroup.Verify(thresholdSignature, thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		assert.True(t, verif, "signature share is not valid")

		// check failure with a random redundant signer
		if threshold > 1 {
			randomDuplicate := rand.Intn(int(threshold)) + 1 // 1 <= duplicate <= threshold
			tmp := signers[randomDuplicate]
			signers[randomDuplicate] = signers[0]
			thresholdSignature, err = BLSReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
			assert.Error(t, err)
			assert.True(t, IsDuplicatedSignerError(err))
			assert.Nil(t, thresholdSignature)
			signers[randomDuplicate] = tmp
		}

		// check with an invalid signature (invalid serialization)
		invalidSig := make([]byte, signatureLengthBLSBLS12381)
		signShares[0] = invalidSig
		thresholdSignature, err = BLSReconstructThresholdSignature(n, threshold, signShares, signers[:threshold+1])
		assert.Error(t, err)
		assert.True(t, IsInvalidSignatureError(err))
		assert.Nil(t, thresholdSignature)
	}
}

func BenchmarkSimpleKeyGen(b *testing.B) {
	n := 60
	seed := make([]byte, SeedMinLenDKG)
	_, err := crand.Read(seed)
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = BLSThresholdKeyGen(n, optimalThreshold(n), seed)
	}
	b.StopTimer()
}
