package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OneToKTestSuite struct {
	suite.Suite
	ee map[int][]*EchoEngine
}

func TestOneToKTestSuite(t *testing.T) {
	suite.Run(t, new(OneToKTestSuite))
}

func (o *OneToKTestSuite) SetupTest() {
	const nodes = 4
	const groups = 2
	//golog.SetAllLoggers(gologging.INFO)
	o.ee = make(map[int][]*EchoEngine)

	subnets, err := CreateSubnets(nodes, groups)
	require.NoError(o.Suite.T(), err)

	// iterates over subnets
	for i := range subnets {
		// iterates over nets in each subnet
		for _, net := range subnets[i] {
			if o.ee[i] == nil {
				o.ee[i] = make([]*EchoEngine, 0)
			}
			e := NewEchoEngine(o.Suite.T(), net, 100, 1)
			o.ee[i] = append(o.ee[i], e)
		}
	}
}
