// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation/mempool"
	"github.com/dapperlabs/flow-go/engine/consensus/propagation/volatile"
	"github.com/dapperlabs/flow-go/module/committee"
	"github.com/dapperlabs/flow-go/network/codec/captain"
	"github.com/dapperlabs/flow-go/network/trickle"
	"github.com/dapperlabs/flow-go/network/trickle/cache"
	"github.com/dapperlabs/flow-go/network/trickle/middleware"
	"github.com/dapperlabs/flow-go/network/trickle/state"
)

func main() {

	// initialize signal catcher
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	// declare configuration parameters
	var (
		identity    string
		entries     []string
		timeout     time.Duration
		connections uint
	)

	// bind configuration parameters
	pflag.StringVarP(&identity, "identity", "i", "consensus1", "identity of our node")
	pflag.StringSliceVarP(&entries, "entries", "e", []string{"consensus-consensus1@localhost:7297"}, "identity table entries for all nodes")
	pflag.DurationVarP(&timeout, "timeout", "t", 1*time.Minute, "how long to try connecting to the network")
	pflag.UintVarP(&connections, "connections", "c", 0, "number of connections to establish to peers")

	// parse configuration parameters
	pflag.Parse()

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// initialize the logger
	log := zerolog.New(os.Stderr).With().Str("identity", identity).Logger()

	log.Info().Msg("flow consensus node starting up")

	// initialize the network codec
	codec := captain.NewCodec()

	log.Info().Msg("initializing engine modules")

	// initialize the node identity list
	com, err := committee.New(entries, identity)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize flow committee")
	}

	// initialize the collection memory pool
	pool, err := mempool.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize engine mempool")
	}

	// initialize the engine cache
	vol, err := volatile.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize engine volatile")
	}

	log.Info().Msg("initializing network modules")

	// initialize trickle state
	state, err := state.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle state")
	}

	// initialize trickle cache
	cache, err := cache.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle cache")
	}

	// initialize trickle middleware
	mw, err := middleware.New(log, codec, connections, com.Me().Address)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle middleware")
	}

	log.Info().Msg("initializing trickle network")

	// initialize trickle network
	net, err := trickle.NewNetwork(log, codec, com, mw, state, cache)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle network")
	}

	log.Info().Msg("starting trickle network")

	// wait to be connected to a few nodes
	select {
	case <-net.Ready():
		log.Info().Msg("trickle network ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start trickle network")
	case <-sig:
		log.Warn().Msg("trickle network start aborted")
		os.Exit(1)
	}

	log.Info().Msg("initializing propagation engine")

	// initialize the propagation engines
	prop, err := propagation.NewEngine(log, net, com, pool, vol)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize propagation engine")
	}

	go propagation.Generate(prop, 20*time.Second)

	log.Info().Msg("starting propagation engine")

	// wait to have our collection mempool bootstrapped
	select {
	case <-prop.Ready():
		log.Info().Msg("propagation engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start propagation engine")
	case <-sig:
		log.Warn().Msg("propagation engine start aborted")
		os.Exit(1)
	}

	log.Info().Msg("flow consensus node startup complete")

	<-sig

	log.Info().Msg("flow consensus node shutting down")

	log.Info().Msg("stopping propagation engine")

	select {
	case <-prop.Done():
		log.Info().Msg("propagation engine shutdown complete")
	case <-time.After(timeout):
		log.Fatal().Msg("could not stop propagation engine")
	case <-sig:
		log.Warn().Msg("propagation engine stop aborted")
		os.Exit(1)
	}

	log.Info().Msg("stopping trickle network")

	select {
	case <-net.Done():
		log.Info().Msg("trickle network shutdown complete")
	case <-time.After(timeout):
		log.Fatal().Msg("could not stop trickle network")
	case <-sig:
		log.Warn().Msg("trickle network stop aborted")
		os.Exit(1)
	}

	log.Info().Msg("flow consensus node shutdown complete")

	os.Exit(0)
}
