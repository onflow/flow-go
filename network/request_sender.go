package network

import "github.com/libp2p/go-libp2p-core/peer"

type RequestSender interface {
	SendRequest(request interface{}, targetID peer.ID) (interface{}, error)
	Close() error
}
