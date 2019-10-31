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
	"github.com/dapperlabs/flow-go/engine/consensus/propagation/volatile"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff"
	"github.com/dapperlabs/flow-go/engine/simulation/generator"
	"github.com/dapperlabs/flow-go/module/committee"
	"github.com/dapperlabs/flow-go/module/mempool"
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

	log := zerolog.New(os.Stderr).With().Str("identity", identity).Logger()

	log.Info().Msg("flow consensus node starting up")

	log.Info().Msg("initializing engine modules")

	// initialize the node identity list
	com, err := committee.New(entries, identity)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize flow committee")
	}

	pool, err := mempool.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize engine mempool")
	}

	vol, err := volatile.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize engine volatile")
	}

	log.Info().Msg("initializing network modules")

	codec := captain.NewCodec()

	state, err := state.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle state")
	}

	cache, err := cache.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle cache")
	}

	mw, err := middleware.New(log, codec, connections, com.Me().Address)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle middleware")
	}

	log.Info().Msg("initializing trickle network")

	net, err := trickle.NewNetwork(log, codec, com, mw, state, cache)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle network")
	}

	log.Info().Msg("starting trickle network")

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
	prop, err := propagation.New(log, net, com, pool, vol)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize propagation engine")
	}

	log.Info().Msg("starting propagation engine")

	select {
	case <-prop.Ready():
		log.Info().Msg("propagation engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start propagation engine")
	case <-sig:
		log.Warn().Msg("propagation engine start aborted")
		os.Exit(1)
	}

	log.Info().Msg("initializing generator engine")

	// initialize the fake collection generation
	gen, err := generator.New(log, prop)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize generator engine")
	}

	log.Info().Msg("starting generator engine")

	select {
	case <-gen.Ready():
		log.Info().Msg("generator engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start generator engine")
	case <-sig:
		log.Warn().Msg("generator engine start aborted")
	}

	cons, err := coldstuff.New(log, net, com, pool)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize coldstuff engine")
	}

	log.Info().Msg("starting coldstuff engine")

	select {
	case <-cons.Ready():
		log.Info().Msg("coldstuff engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start coldstuff engine")
	case <-sig:
		log.Warn().Msg("coldstuff engine start aborted")
		os.Exit(1)
	}

	log.Info().Msg("flow consensus node startup complete")

	<-sig

	log.Info().Msg("flow consensus node shutting down")

	log.Info().Msg("stopping generator engine")

	select {
	case <-gen.Done():
		log.Info().Msg("generator engine shutdown complete")
	case <-time.After(timeout):
		log.Fatal().Msg("could not stop generator engine")
	case <-sig:
		log.Warn().Msg("generator engine stop aborted")
	}

	log.Info().Msg("stopping coldstuff engine")

	select {
	case <-cons.Done():
		log.Info().Msg("coldstuff engine shutdown complete")
	case <-time.After(timeout):
		log.Fatal().Msg("could not stop coldstuff engine")
	case <-sig:
		log.Warn().Msg("coldstuff engine stop aborted")
		os.Exit(1)
	}

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
