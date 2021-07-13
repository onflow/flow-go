package relay

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite

	engine *Engine
}

func (suite *Suite) SetupTest() {
	eng, err := New(
	// TODO
	)
	suite.Require().Nil(err)

	suite.engine = eng
}

func TestRelayEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}
