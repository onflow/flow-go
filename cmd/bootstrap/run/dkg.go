package run

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/signature"
)

func RunDKG(n int, seeds [][]byte) (model.DKGData, error) {

	if n != len(seeds) {
		return model.DKGData{}, fmt.Errorf("n needs to match the number of seeds (%v != %v)", n, len(seeds))
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

type message struct {
	orig int
	data []byte
}

// PrivateSend sends a message from one node to another
func (proc *localDKGProcessor) PrivateSend(dest int, data []byte) {
	newMsg := &message{proc.current, data}
	proc.chans[dest] <- newMsg
}

// Broadcast a message from one node to all nodes
func (proc *localDKGProcessor) Broadcast(data []byte) {
	newMsg := &message{proc.current, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

// Blacklist a node
func (proc *localDKGProcessor) Blacklist(node int) {
}

// FlagMisbehavior flags a node for misbehaviour
func (proc *localDKGProcessor) FlagMisbehavior(node int, logData string) {
}

// dkgRunChan simulates processing incoming messages by a node
// it assumes proc.dkg is already running
func dkgRunChan(proc *localDKGProcessor, sync *sync.WaitGroup, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			err := proc.dkg.HandleMsg(newMsg.orig, newMsg.data)
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
