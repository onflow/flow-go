// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/engine/simulation/coldstuff"
	"github.com/dapperlabs/flow-go/engine/simulation/generator"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/trickle"
	"github.com/dapperlabs/flow-go/network/trickle/middleware"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

func main() {

	// initialize signal catcher
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	// declare configuration parameters
	var (
		nodeID      string
		entries     []string
		timeout     time.Duration
		connections uint
		datadir     string
		level       string
	)

	// bind configuration parameters
	pflag.StringVarP(&nodeID, "nodeid", "n", "node1", "identity of our node")
	pflag.StringSliceVarP(&entries, "entries", "e", []string{"consensus-node1@address1=1000"}, "identity table entries for all nodes")
	pflag.DurationVarP(&timeout, "timeout", "t", 1*time.Minute, "how long to try connecting to the network")
	pflag.UintVarP(&connections, "connections", "c", 0, "number of connections to establish to peers")
	pflag.StringVarP(&datadir, "datadir", "d", "data", "directory to store the protocol state")
	pflag.StringVarP(&level, "loglevel", "l", "info", "level for logging output")

	// parse configuration parameters
	pflag.Parse()

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	log.Info().Msg("flow consensus node starting up")

	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(os.Stderr).With().Timestamp().Str("node_id", nodeID).Logger()

	// parse config log level and apply to logger
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		log.Fatal().Err(err).Msg("invalid log level")
	}
	log.Level(lvl)

	log.Info().Msg("initializing engine modules")

	db, err := badger.Open(badger.DefaultOptions(datadir).WithLogger(nil))
	if err != nil {
		log.Fatal().Err(err).Msg("could not open key-value store")
	}

	state, err := protocol.NewState(db)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize flow committee")
	}

	var ids flow.IdentityList
	for _, entry := range entries {
		id, err := flow.ParseIdentity(entry)
		if err != nil {
			log.Fatal().Err(err).Str("entry", entry).Msg("could not parse identity")
		}
		ids = append(ids, id)
	}

	err = state.Mutate().Bootstrap(flow.Genesis(ids))
	if err != nil {
		log.Fatal().Err(err).Msg("could not bootstrap protocol state")
	}

	var trueID flow.Identifier
	copy(trueID[:], []byte(nodeID))
	id, err := state.Final().Identity(trueID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not get identity")
	}

	me, err := local.New(id)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize local")
	}

	pool, err := mempool.New()
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize engine mempool")
	}

	log.Info().Msg("initializing network stack")

	codec := json.NewCodec()

	mw, err := middleware.New(log, codec, connections, me.Address())
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle middleware")
	}

	net, err := trickle.NewNetwork(log, codec, state, me, mw)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize trickle network")
	}

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

	prop, err := propagation.New(log, net, state, me, pool)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize propagation engine")
	}

	select {
	case <-prop.Ready():
		log.Info().Msg("propagation engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start propagation engine")
	case <-sig:
		log.Warn().Msg("propagation engine start aborted")
		os.Exit(1)
	}

	log.Info().Msg("initializing coldstuff engine")

	cold, err := coldstuff.New(log, net, state, me, pool)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize coldstuff engine")
	}

	select {
	case <-cold.Ready():
		log.Info().Msg("coldstuff engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start coldstuff engine")
	case <-sig:
		log.Warn().Msg("coldstuff engine start aborted")
		os.Exit(1)
	}

	log.Info().Msg("initializing generator engine")

	gen, err := generator.New(log, prop)
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize generator engine")
	}

	select {
	case <-gen.Ready():
		log.Info().Msg("generator engine ready")
	case <-time.After(timeout):
		log.Fatal().Msg("could not start generator engine")
	case <-sig:
		log.Warn().Msg("generator engine start aborted")
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
	case <-cold.Done():
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

	log.Info().Msg("stopping network stack")

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
