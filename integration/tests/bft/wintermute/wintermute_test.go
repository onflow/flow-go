package wintermute

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestWintermute(t *testing.T) {
	suite.Run(t, new(WintermuteTestSuite))
}

type WintermuteTestSuite struct {
	Suite
}

func (suite *WintermuteTestSuite) TestDummyOrchestrator() {

}
