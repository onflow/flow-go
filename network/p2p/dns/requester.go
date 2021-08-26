package dns

import (
	"fmt"
	"sync"

	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/onflow/flow-go/network"
)

type requester struct {
	sync.Mutex
	pendingIPRequests  map[string]struct{}
	pendingTXTRequests map[string]struct{}
	res                madns.BasicResolver // underlying resolver
	ipHandler          network.DNSIPHandlerFunc
	txtHandler         network.DNSTXTHandlerFunc
}

func newRequester(res madns.BasicResolver, ipHandler network.DNSIPHandlerFunc, txtHandler network.DNSTXTHandlerFunc) (*requester, error) {
	if ipHandler == nil {
		return nil, fmt.Errorf("could not create dns requester with missing ip handler function")
	}
	if txtHandler == nil {
		return nil, fmt.Errorf("could not create dns requester with missing txt handler function")
	}

	return &requester{
		pendingIPRequests:  make(map[string]struct{}),
		pendingTXTRequests: make(map[string]struct{}),
		res:                res,
		ipHandler:          ipHandler,
		txtHandler:         txtHandler,
	}, nil
}
