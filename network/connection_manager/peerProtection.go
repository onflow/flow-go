package connection_manager

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
)

type PeerProtection struct {
	log zerolog.Logger
	// map to track stream setup progress for each peer
	// stream setup involves creating a connection (if none exist) with the remote and then creating a stream on that connection.
	// This map is used to make sure that both these steps occur atomically.
	streamSetupInProgressCnt map[peer.ID]int
	// mutex for the stream setup map
	streamSetupMapLk sync.RWMutex
}

func NewPeerProtection(log zerolog.Logger) *PeerProtection {
	return &PeerProtection{
		log:                      log,
		streamSetupInProgressCnt: make(map[peer.ID]int),
	}
}

// ProtectPeer increments the stream setup count for the peer.ID
func (p *PeerProtection) ProtectPeer(id peer.ID) {
	p.streamSetupMapLk.Lock()
	defer p.streamSetupMapLk.Unlock()

	p.streamSetupInProgressCnt[id]++

	p.log.Trace().
		Str("peer_id", id.String()).
		Int("stream_setup_in_progress_cnt", p.streamSetupInProgressCnt[id]).
		Msg("protected from connection pruning")
}

// UnprotectPeer decrements the stream setup count for the peer.ID.
// If the count reaches zero, the id is removed from the map
func (p *PeerProtection) UnprotectPeer(id peer.ID) {
	p.streamSetupMapLk.Lock()
	defer p.streamSetupMapLk.Unlock()

	cnt := p.streamSetupInProgressCnt[id]
	cnt = cnt - 1

	defer func() {
		p.log.Trace().
			Str("peer_id", id.String()).
			Int("stream_setup_in_progress_cnt", cnt).
			Msg("unprotected from connection pruning")
	}()

	if cnt <= 0 {
		delete(p.streamSetupInProgressCnt, id)
		return
	}
	p.streamSetupInProgressCnt[id] = cnt
}

// IsProtected returns true is there is at least one stream setup in progress for the given peer.ID else false
func (p *PeerProtection) IsProtected(id peer.ID, _ string) (protected bool) {
	p.streamSetupMapLk.RLock()
	defer p.streamSetupMapLk.RUnlock()

	return p.streamSetupInProgressCnt[id] > 0
}
