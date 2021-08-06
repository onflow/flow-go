package dns

import (
	"math/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
)

func TestResolver(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	_, err := NewResolver(metrics.NewNoopCollector(), WithBasicResolver(&basicResolver))
	require.NoError(t, err)

	basicResolver.On("LookupIPAddr", "example1.com").Return([]net.IPAddr{})
}

func netIPAddrFixture() []net.IPAddr {
	token := make([]byte, 4)
	rand.Read(token)

	ip := net.IPAddr{
		IP:   net.IPv4(token[0], token[1], token[2], token[3]),
		Zone: "flow",
	}

	return []net.IPAddr{ip}
}
