package test

import (
	"sync"
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

// portAllocator is a test helper type keeping track of allocated free ports for testing.
type portAllocator struct {
	sync.Mutex
	allocatedPorts map[int]struct{} // keeps track of ports allocated to different tests
}

func newPortAllocator() *portAllocator {
	return &portAllocator{
		allocatedPorts: make(map[int]struct{}),
	}
}

// getFreePorts finds `n` free ports on the machine and marks them as allocated.
func (p *portAllocator) getFreePorts(t *testing.T, n int) []int {
	p.Lock()
	defer p.Unlock()

	ports := make([]int, n)
	// keeps track of discovered ports
	for count := 0; count < n; {
		// get free ports
		freePorts, err := freeport.GetFreePorts(1)
		require.NoError(t, err)
		port := freePorts[0]

		if _, ok := p.allocatedPorts[port]; ok {
			// port has already been allocated
			continue
		}

		// records port address and mark it as allocated
		ports[count] = port
		p.allocatedPorts[port] = struct{}{}
		count++
	}

	return ports
}
