package dkg

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/module/signature"
)

// RunDKG simulates a distributed DKG protocol by running the protocol locally
// and generating the DKG output info
func RunDKG(n int, seeds [][]byte) (model.DKGData, error) {

	if n != len(seeds) {
		return model.DKGData{}, fmt.Errorf("n needs to match the number of seeds (%v != %v)", n, len(seeds))
	}

	// separate the case whith one node
	if n == 1 {
		sk, pk, pkGroup, err := thresholdSignKeyGenOneNode(seeds[0])
		if err != nil {
			return model.DKGData{}, fmt.Errorf("run dkg failed: %w", err)
		}

		dkgData := model.DKGData{
			PrivKeyShares: sk,
			PubGroupKey:   pkGroup,
			PubKeyShares:  pk,
		}

		return dkgData, nil
	}

	processors := make([]localDKGProcessor, 0, n)

	// create the message channels for node communication
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create processors for all nodes
	for i := 0; i < n; i++ {
		processors = append(processors, localDKGProcessor{
			current: i,
			chans:   chans,
		})
	}

	// create DKG instances for all nodes
	for i := 0; i < n; i++ {
		var err error
		processors[i].dkg, err = crypto.NewJointFeldman(n,
			signature.RandomBeaconThreshold(n), i, &processors[i])
		if err != nil {
			return model.DKGData{}, err
		}
	}

	var wg sync.WaitGroup
	phase := 0

	// start DKG in all nodes
	// start listening on the channels
	wg.Add(n)
	for i := 0; i < n; i++ {
		// start dkg could also run in parallel
		// but they are run sequentially to avoid having non-deterministic
		// output (the PRG used is common)
		err := processors[i].dkg.Start(seeds[i])
		if err != nil {
			return model.DKGData{}, err
		}
		go dkgRunChan(&processors[i], &wg, phase)
	}
	phase++

	// sync the two timeouts and start the next phase
	for ; phase <= 2; phase++ {
		wg.Wait()
		wg.Add(n)
		for i := 0; i < n; i++ {
			go dkgRunChan(&processors[i], &wg, phase)
		}
	}

	// synchronize the main thread to end all DKGs
	wg.Wait()

	skShares := make([]crypto.PrivateKey, 0, n)

	for _, processor := range processors {
		skShares = append(skShares, processor.privkey)
	}

	dkgData := model.DKGData{
		PrivKeyShares: skShares,
		PubGroupKey:   processors[0].pubgroupkey,
		PubKeyShares:  processors[0].pubkeys,
	}

	return dkgData, nil
}

// localDKGProcessor implements DKGProcessor interface
type localDKGProcessor struct {
	current     int
	dkg         crypto.DKGState
	chans       []chan *message
	privkey     crypto.PrivateKey
	pubgroupkey crypto.PublicKey
	pubkeys     []crypto.PublicKey
}

const (
	broadcast int = iota
	private
)

type message struct {
	orig    int
	channel int
	data    []byte
}

// PrivateSend sends a message from one node to another
func (proc *localDKGProcessor) PrivateSend(dest int, data []byte) {
	newMsg := &message{proc.current, private, data}
	proc.chans[dest] <- newMsg
}

// Broadcast a message from one node to all nodes
func (proc *localDKGProcessor) Broadcast(data []byte) {
	newMsg := &message{proc.current, broadcast, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

// Disqualify a node
func (proc *localDKGProcessor) Disqualify(node int, log string) {
}

// FlagMisbehavior flags a node for misbehaviour
func (proc *localDKGProcessor) FlagMisbehavior(node int, log string) {
}

// dkgRunChan simulates processing incoming messages by a node
// it assumes proc.dkg is already running
func dkgRunChan(proc *localDKGProcessor, sync *sync.WaitGroup, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			var err error
			if newMsg.channel == private {
				err = proc.dkg.HandlePrivateMsg(newMsg.orig, newMsg.data)
			} else {
				err = proc.dkg.HandleBroadcastMsg(newMsg.orig, newMsg.data)
			}
			if err != nil {
				log.Fatal().Err(err).Msg("failed to receive DKG mst")
			}
		// if timeout, stop and finalize
		case <-time.After(1 * time.Second):
			switch phase {
			case 0:
				err := proc.dkg.NextTimeout()
				if err != nil {
					log.Fatal().Err(err).Msg("failed to wait for next timeout")
				}
			case 1:
				err := proc.dkg.NextTimeout()
				if err != nil {
					log.Fatal().Err(err).Msg("failed to wait for next timeout")
				}
			case 2:
				privkey, pubgroupkey, pubkeys, err := proc.dkg.End()
				if err != nil {
					log.Fatal().Err(err).Msg("end dkg error should be nit")
				}
				if privkey == nil {
					log.Fatal().Msg("privkey was nil")
				}

				proc.privkey = privkey
				proc.pubgroupkey = pubgroupkey
				proc.pubkeys = pubkeys
			}
			sync.Done()
			return
		}
	}
}

// RunFastKG is an alternative to RunDKG that runs much faster by using a centralized threshold signature key generation.
func RunFastKG(n int, seed []byte) (model.DKGData, error) {

	if n == 1 {
		sk, pk, pkGroup, err := thresholdSignKeyGenOneNode(seed)
		if err != nil {
			return model.DKGData{}, fmt.Errorf("fast KeyGen failed: %w", err)
		}

		dkgData := model.DKGData{
			PrivKeyShares: sk,
			PubGroupKey:   pkGroup,
			PubKeyShares:  pk,
		}
		return dkgData, nil
	}

	skShares, pkShares, pkGroup, err := crypto.ThresholdSignKeyGen(int(n),
		signature.RandomBeaconThreshold(int(n)), seed)
	if err != nil {
		return model.DKGData{}, fmt.Errorf("fast KeyGen failed: %w", err)
	}

	dkgData := model.DKGData{
		PrivKeyShares: skShares,
		PubGroupKey:   pkGroup,
		PubKeyShares:  pkShares,
	}

	return dkgData, nil
}

// simulates DKG with one single node
func thresholdSignKeyGenOneNode(seed []byte) ([]crypto.PrivateKey, []crypto.PublicKey, crypto.PublicKey, error) {
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("KeyGen with one node failed: %w", err)
	}
	pk := sk.PublicKey()
	return []crypto.PrivateKey{sk},
		[]crypto.PublicKey{pk},
		pk,
		nil
}
