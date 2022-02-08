package network

import "github.com/libp2p/go-libp2p-core/peer"

type MessageSender interface {
	SendMessage(message interface{}, targetID peer.ID) error
	Close() error
}
