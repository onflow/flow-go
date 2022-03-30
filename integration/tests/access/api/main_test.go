package api

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/access"
	"github.com/stretchr/testify/suite"
)

func TestAccessSuite(t *testing.T) {
	suite.Run(t, new(access.AccessSuite))
}
