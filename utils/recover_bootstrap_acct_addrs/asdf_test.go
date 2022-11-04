package main

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

func TestASDf(t *testing.T) {
	addr := flow.HexToAddress("f698fb359d4c6a72")
	idx, _ := flow.Sandboxnet.Chain().IndexFromAddress(addr)
	fmt.Println(addr, idx)

	addr = flow.HexToAddress("e0cc9e0604486013")
	idx, _ = flow.Sandboxnet.Chain().IndexFromAddress(addr)
	fmt.Println(addr, idx)
}

func TestEsdf(t *testing.T) {
	for i := 0; i < 5; i++ {
		addr, _ := flow.Sandboxnet.Chain().AddressAtIndex(uint64(i))
		fmt.Println(i, addr.HexWithPrefix())
	}
}

// access-001.sandboxnet2.nodes.onflow.org:9000
