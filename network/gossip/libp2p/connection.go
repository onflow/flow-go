// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"errors"
	"sync"

	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/mux"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-yamux"
	"github.com/rs/zerolog"
)

// Connection represents a direct connection to another peer on the flow
// network.
type Connection struct {
	log    zerolog.Logger
	stream libp2pnetwork.Stream
	once   *sync.Once
	done   chan struct{}
}

// NewConnection creates a new connection to a peer on the flow network, using
// the provided encoder and decoder to read and write messages.
func NewConnection(log zerolog.Logger, stream libp2pnetwork.Stream) *Connection {

	log = log.With().
		Str("local_addr", stream.Conn().LocalPeer().String()).
		Str("remote_addr", stream.Conn().RemotePeer().String()).
		Logger()

	c := Connection{
		log:    log,
		stream: stream,
		once:   &sync.Once{},
		done:   make(chan struct{}),
	}

	return &c
}

// stop closes the done channel and resets the stream
func (c *Connection) stop() {
	c.once.Do(func() {
		close(c.done)
		// Close the underlying libp2p stream (only log errors other than the expected ErrReset error for the mux type)
		if err := helpers.FullClose(c.stream); err != nil && !errors.Is(err, mux.ErrReset) && !errors.Is(err, yamux.ErrConnectionReset) {
			c.log.Err(err).Msg("error resetting connection stream")
		}
	})
}
