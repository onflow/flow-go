package dkg

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestDKGt(t *testing.T) {
	suite.Run(t, new(DKGSuite))
}
