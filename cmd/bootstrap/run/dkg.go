package run

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/crypto"
)

type DKGParticipant struct {
	Priv       crypto.PrivateKey
	GroupIndex int
}

type DKGData struct {
	PubGroupKey  crypto.PublicKey
	PubKeys      []crypto.PublicKey
	Participants []DKGParticipant
}

func RunDKG(n int, seeds [][]byte) (DKGData, error) {
	if n != len(seeds) {
		return DKGData{}, fmt.Errorf("n needs to match the number of seeds (%v != %v)", n, len(seeds))
	}

	processors := make([]LocalDKGProcessor, 0, n)

	// create the message channels for node communication
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create processors for all nodes
	for i := 0; i < n; i++ {
		processors = append(processors, LocalDKGProcessor{
			current: i,
			chans:   chans,
			// msgType: dkgType,
		})
	}

	// create DKG instances for all nodes
	for i := 0; i < n; i++ {
		var err error
		processors[i].dkg, err = crypto.NewDKG(crypto.JointFeldman, n, i, &processors[i], 0)
		if err != nil {
			return DKGData{}, err
		}
	}

	var sync sync.WaitGroup
	phase := 0

	// start DKG in all nodes
	// start listening on the channels
	sync.Add(n)
	for i := 0; i < n; i++ {
		// start dkg could also run in parallel
		// but they are run sequentially to avoid having non-deterministic
		// output (the PRG used is common)
		err := processors[i].dkg.StartDKG(seeds[i])
		if err != nil {
			return DKGData{}, err
		}
		go dkgRunChan(&processors[i], &sync, phase)
	}
	phase++

	// sync the two timeouts and start the next phase
	for ; phase <= 2; phase++ {
		sync.Wait()
		sync.Add(n)
		for i := 0; i < n; i++ {
			go dkgRunChan(&processors[i], &sync, phase)
		}
	}

	// synchronize the main thread to end all DKGs
	sync.Wait()

	dkgData := DKGData{Participants: make([]DKGParticipant, 0, n), PubKeys: make([]crypto.PublicKey, 0, n)}
	for _, processor := range processors {
		dkgData.Participants = append(dkgData.Participants, DKGParticipant{
			Priv:       processor.privkey,
			GroupIndex: processor.current,
		})
		dkgData.PubGroupKey = processor.pubgroupkey
	}
	for _, processor := range processors {
		dkgData.PubKeys = append(dkgData.PubKeys, processor.privkey.PublicKey())
	}

	return dkgData, nil
}

// LocalDKGProcessor implements DKGProcessor interface
type LocalDKGProcessor struct {
	current     int
	dkg         crypto.DKGstate
	chans       []chan *message
	privkey     crypto.PrivateKey
	pubgroupkey crypto.PublicKey
}

type message struct {
	orig int
	// msgType int
	data []byte
}

// Send a message from one node to another
func (proc *LocalDKGProcessor) Send(dest int, data []byte) {
	log.Debug().Str("data", fmt.Sprintf("%x", data)).Msgf("%d Sending to %d", proc.current, dest)
	newMsg := &message{proc.current, data}
	proc.chans[dest] <- newMsg
}

// Broadcast a message from one node to all nodes
func (proc *LocalDKGProcessor) Broadcast(data []byte) {
	log.Debug().Str("data", fmt.Sprintf("%x", data)).Msgf("%d Broadcasting", proc.current)
	newMsg := &message{proc.current, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

// Blacklist a node
func (proc *LocalDKGProcessor) Blacklist(node int) {
	log.Debug().Msgf("%d wants to blacklist %d", proc.current, node)
}

// FlagMisbehavior flags a node for misbehaviour
func (proc *LocalDKGProcessor) FlagMisbehavior(node int, logData string) {
	log.Debug().Msgf("%d flags a misbehavior from %d: %s", proc.current, node, logData)
}

// dkgRunChan simulates processing incoming messages by a node
// it assumes proc.dkg is already running
func dkgRunChan(proc *LocalDKGProcessor, sync *sync.WaitGroup, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debug().Str("data", fmt.Sprintf("%x", newMsg.data)).Msgf("%d Receiving from %d", proc.current, newMsg.orig)
			err := proc.dkg.ReceiveDKGMsg(newMsg.orig, newMsg.data)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to receive DKG mst")
			}
		// if timeout, stop and finalize
		case <-time.After(1 * time.Second):
			switch phase {
			case 0:
				log.Debug().Msgf("%d shares phase ended", proc.current)
				err := proc.dkg.NextTimeout()
				if err != nil {
					log.Fatal().Err(err).Msg("failed to wait for next timeout")
				}
			case 1:
				log.Debug().Msgf("%d complaints phase ended", proc.current)
				err := proc.dkg.NextTimeout()
				if err != nil {
					log.Fatal().Err(err).Msg("failed to wait for next timeout")
				}
			case 2:
				log.Debug().Msgf("%d dkg ended", proc.current)
				privkey, pubgroupkey, _, err := proc.dkg.EndDKG()
				if err != nil {
					log.Fatal().Err(err).Msg("end dkg error should be nit")
				}
				if privkey == nil {
					log.Fatal().Msg("privkey was nil")
				}

				proc.privkey = privkey
				proc.pubgroupkey = pubgroupkey
			}
			sync.Done()
			return
		}
	}
}
