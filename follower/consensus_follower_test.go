package follower

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger
	net      *module.Network
}

func TestConsensusFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.New(os.Stderr)
	suite.net = new(module.Network)
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
}

func (suite *Suite) TestFollowerReceivesBlocks() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		options := node_builder.WithBaseOptions([]cmd.Option{cmd.WithDB(db)})
		nodeBuilder := node_builder.NewUnstakedAccessNodeBuilder(node_builder.FlowAccessNode(options))
		nodeBuilder.Initialize()
		nodeBuilder.Build()
		<-nodeBuilder.Ready()
	})
}
