package engine

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

type ResendEngine struct {
	unit            *engine.Unit
	network         *libp2p.Network
	me              module.Local
	storage         cmd.Storage
	logger          zerolog.Logger
	conduit         network.Conduit
	target          flow.Identifier
	flagHeightStart uint64
	flagHeightEnd   uint64
	flagDelayMs     uint16
}

func New(logger zerolog.Logger, network *libp2p.Network, me module.Local, storage cmd.Storage,
	flagHeightStart uint64, flagHeightEnd uint64, flagDelayMs uint16, flagTarget string) *ResendEngine {

	identifier, err := flow.HexStringToIdentifier(flagTarget)
	if err != nil {
		logger.Fatal().Msg("cannot decode target identifier")
	}

	e := &ResendEngine{
		unit:    engine.NewUnit(),
		network: network,
		me:      me,
		storage: storage,
		logger:  logger,
		flagHeightEnd: flagHeightEnd,
		flagHeightStart: flagHeightStart,
		flagDelayMs: flagDelayMs,
		target: identifier,
	}

	conduit, err := network.Register(engine.PushBlocks, e)
	if err != nil {
		e.logger.Fatal().Msg("start cannot be higher then end")
	}

	e.conduit = conduit

	return e
}

// Engine boilerplate
func (e *ResendEngine) SubmitLocal(event interface{}) {
	panic("no submitLocal expected")
}

func (e *ResendEngine) Submit(originID flow.Identifier, event interface{}) {
	panic("no submit expected")

}

func (e *ResendEngine) ProcessLocal(event interface{}) error {
	panic("no processlocal expected")
}

func (e *ResendEngine) Process(originId flow.Identifier, event interface{}) error {
	panic("no processlocal expected")
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *ResendEngine) Ready() <-chan struct{} {

	go e.SendBlocks()

	return e.unit.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *ResendEngine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *ResendEngine) SendBlocks() {

	if e.flagHeightEnd < e.flagHeightStart {
		e.logger.Fatal().Msg("start cannot be higher then end")
	}

	for height := e.flagHeightStart; height <= e.flagHeightEnd; height++ {

		block, err := e.storage.Blocks.ByHeight(height)

		if err != nil {
			e.logger.Fatal().Err(err).Uint64("block_height", height).Msg("cannot find header")
		}

		proposal := messages.BlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}

		err = e.conduit.Submit(&proposal, e.target)
		if err != nil {
			e.logger.Fatal().Err(err).Uint64("block_height", height).Msg("submit")
		}

		d := time.Duration(e.flagDelayMs) * time.Millisecond
		time.Sleep(d)
	}
}
