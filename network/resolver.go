package network

import (
	"context"
	"net"
)

// BasicResolver is a low level interface for DNS resolution
// Note: this is the resolver interface that libp2p expects.
// We keep a copy of it here for mock generation.
// https://github.com/multiformats/go-multiaddr-dns/blob/master/resolve.go
type BasicResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
	LookupTXT(context.Context, string) ([]string, error)
}
