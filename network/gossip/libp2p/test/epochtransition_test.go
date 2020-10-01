package test

import (
	"os"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p"
)

type EpochTransitionTestSuite struct {
	suite.Suite
	ConduitWrapper                      // used as a wrapper around conduit methods
	nets           []*libp2p.Network    // used to keep track of the networks
	mws            []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids            flow.IdentityList    // used to keep track of the identifiers associated with networks
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(EchoEngineTestSuite))
}

func (m *EpochTransitionTestSuite) SetupTest() {
	// defines total number of nodes in our network (minimum 3 needed to use 1-k messaging)
	const count = 10
	const cacheSize = 100
	golog.SetAllLoggers(golog.LevelInfo)

	m.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := createMiddleware(logger, m.ids)
	require.NoError(m.Suite.T(), err)
	m.mws = mws

	nets, err := createNetworks(logger, m.mws, m.ids, cacheSize, false, nil, nil)
	require.NoError(m.Suite.T(), err)
	m.nets = nets
}

// TearDownTest closes the networks within a specified timeout
func (m *EpochTransitionTestSuite) TearDownTest() {
	for _, net := range m.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			m.Suite.Fail("could not stop the network")
		}
	}
}
