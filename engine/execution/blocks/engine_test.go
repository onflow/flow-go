package blocks

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	module "github.com/dapperlabs/flow-go/module/mocks"
	network "github.com/dapperlabs/flow-go/network/mocks"
	storage "github.com/dapperlabs/flow-go/storage/mocks"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockStorage(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	net := module.NewMockNetwork(ctrl)

	// initialize the mocks and engine
	conduit := network.NewMockConduit(ctrl)
	me := module.NewMockLocal(ctrl)

	blocks := storage.NewMockBlocks(ctrl)

	log := zerolog.Logger{}

	var engine *Engine

	net.EXPECT().Register(gomock.Any(), gomock.AssignableToTypeOf(engine)).Return(conduit, nil)

	engine, err := New(log, net, me, blocks)
	require.NoError(t, err)

	identifier := unittest.IdentifierFixture()

	block := unittest.BlockFixture()

	blocks.EXPECT().Save(gomock.Eq(&block))

	err = engine.Process(identifier, block)
	assert.NoError(t, err)
}
