package p2p

import (
	"bufio"
	"context"
	"fmt"

	ggio "github.com/gogo/protobuf/io"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

// TODO: So we should completely separate pubsub from direct communication
// For direct, we can register a MessageHandler and receive back a StreamCreator
// StreamCreator in turn can create Streams, which can SendMessage, ReceiveMessage, and Closef

type RequestSender struct {
	codec   network.Codec
	channel network.Channel
}

var _ network.RequestSender = (*RequestSender)(nil)

func (m *RequestSender) SendRequest(ctx context.Context, event interface{}, targetID peer.ID) (interface{}, error) {
	payload, err := m.codec.Encode(event)

	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	msg := &message.DirectMessage{
		payload: payload,
	}

	maxMsgSize := maxDirectMsgSize(msg)

	if msg.Size() > maxMsgSize {
		return nil, fmt.Errorf("message size %d exceeds configured max message size %d", msg.Size(), maxMsgSize)
	}

	// protect the underlying connection from being inadvertently pruned by the peer manager while the stream and
	// connection creation is being attempted, and remove it from protected list once stream created.
	tag := fmt.Sprintf("%v:%v", m.channel, msg.Type)
	m.libP2PNode.connMgr.Protect(peerID, tag)
	defer m.libP2PNode.connMgr.Unprotect(peerID, tag)

	// create new stream
	// (streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the receiver
	stream, err := m.libP2PNode.CreateStream(ctx, peerID)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s: %w", targetID, err)
	}

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err = writer.WriteMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %s: %w", targetID, err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", targetID, err)
	}

	// close the stream immediately
	err = stream.Close()
	if err != nil {
		return fmt.Errorf("failed to close the stream for %s: %w", targetID, err)
	}

	channel := metrics.ChannelOneToOne
	if _, isStaked := m.ov.Identities().ByNodeID(targetID); !isStaked {
		channel = metrics.ChannelOneToOneUnstaked
	}
	// OneToOne communication metrics are reported with topic OneToOne
	m.metrics.NetworkMessageSent(msg.Size(), channel, msg.Type)

	return nil
}
