package p2p

import (
	"sync"

	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
)

type Responder struct {
	mu sync.Mutex
}

func NewResponder(stream libp2pnetwork.Stream) *Responder {
	return &Responder{}
}

func (r *Responder) SetError(err error) {

}

func (r *Responder) SetResponse(resp interface{}) {

}
