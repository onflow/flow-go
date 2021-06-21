package epochs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestEpochs(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) TestViewsProgress() {
	proposal := s.BlockState.WaitForSealedView(s.T(), 5)
	fmt.Printf("XXX proposal height: %d, view: %d\n", proposal.Header.Height, proposal.Header.View)
}
