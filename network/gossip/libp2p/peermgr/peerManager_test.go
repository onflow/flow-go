package peermgr

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type PeerManagerTestSuite struct {
	suite.Suite
	log zerolog.Logger
	pm *PeerManager
	ctx context.Context
}

func TestPeerManagerTestSuite(t *testing.T) {
	suite.Run(t, new(PeerManagerTestSuite))
}

func (ts *PeerManagerTestSuite) SetupTest() {
	ts.log = ts.log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	ts.ctx = context.Background()
	ts.pm = NewPeerManager(ts.ctx, ts.log, nil)
}

func (ts *PeerManagerTestSuite) TearDownTest() {
}


func (ts *PeerManagerTestSuite) TestSomething() {
	PeerUpdateInterval = 1 * time.Second
	ts.pm.Start()
	ts.pm.RequestPeerUpdate()
	ts.pm.RequestPeerUpdate()
	time.Sleep(time.Second * 10)
}
