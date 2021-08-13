package p2p

import (
	"context"
	"fmt"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/stretchr/testify/require"
)

func TestBasicSubscriptionFilter(t *testing.T) {
	golog.SetAllLoggers(golog.LevelDebug)
	ctx := context.Background()
	host1, err := libp2p.New(ctx)
	require.NoError(t, err)
	host2, err := libp2p.New(ctx)
	require.NoError(t, err)
	host3, err := libp2p.New(ctx)
	require.NoError(t, err)

	require.NoError(t, host1.Connect(ctx, *host.InfoFromHost(host2)))
	require.NoError(t, host1.Connect(ctx, *host.InfoFromHost(host3)))


	topicname1 := "testtopic1"
	topicname2 := "testtopic2"

	filter := &Filter{
		allowedIDs: make(map[peer.ID]struct{}),
		topic: topicname2,
	}
	filter.allowedIDs[host1.ID()] = struct{}{}
	filter.allowedIDs[host2.ID()] = struct{}{}


	ps1, err := pubsub.NewGossipSub(ctx, host1, pubsub.WithSubscriptionFilter(filter))
	require.NoError(t, err)
	ps2, err := pubsub.NewGossipSub(ctx, host2, pubsub.WithSubscriptionFilter(filter))
	require.NoError(t, err)
	ps3, err := pubsub.NewGossipSub(ctx, host3)
	require.NoError(t, err)


	topic1, err := ps1.Join(topicname1)
	require.NoError(t, err)
	_, err = topic1.Subscribe()
	require.NoError(t, err)
	topic1, err = ps1.Join(topicname2)
	require.NoError(t, err)
	_, err = topic1.Subscribe()
	require.NoError(t, err)

	topic2, err := ps2.Join(topicname1)
	require.NoError(t, err)
	_, err = topic2.Subscribe()
	require.NoError(t, err)
	topic2, err = ps2.Join(topicname2)
	require.NoError(t, err)
	subscriberHost2Topic2, err := topic2.Subscribe()
	require.NoError(t, err)

	topic3, err := ps3.Join(topicname1)
	require.NoError(t, err)
	_, err = topic3.Subscribe()
	require.NoError(t, err)
	wrongTopic, err := ps3.Join(topicname2)
	require.NoError(t, err)
	_, err = wrongTopic.Subscribe()
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	fmt.Printf("host1: %s, host2: %s, host3 :%s\n", host1.ID(), host2.ID(), host3.ID())
	//fmt.Print(host1.Peerstore().Peers())
	//fmt.Print(host2.Peerstore().Peers())
	fmt.Print("host 1 peers\n")
	fmt.Printf("\t For %s", topicname1)
	fmt.Println(ps1.ListPeers(topicname1))
	fmt.Printf("\t For %s", topicname2)
	fmt.Println(ps1.ListPeers(topicname2))

	fmt.Print("host 2 peers\n")
	fmt.Printf("\t For %s", topicname1)
	fmt.Println(ps2.ListPeers(topicname1))
	fmt.Printf("\t For %s", topicname2)
	fmt.Println(ps2.ListPeers(topicname2))

	fmt.Print("host 3 peers\n")
	fmt.Printf("\t For %s", topicname1)
	fmt.Println(ps3.ListPeers(topicname1))
	fmt.Printf("\t For %s", topicname2)
	fmt.Println(ps3.ListPeers(topicname2))

	err = wrongTopic.Publish(ctx, []byte("hello"))
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	msg, err := subscriberHost2Topic2.Next(ctx)
	require.NoError(t, err)
	fmt.Printf(" message recvd on topic %s from peer %s\n", *msg.Topic, msg.ReceivedFrom.String())
	fmt.Println(msg)

	fmt.Print("host 2 peers\n")
	fmt.Printf("\t For %s", topicname1)
	fmt.Println(ps2.ListPeers(topicname1))
	fmt.Printf("\t For %s", topicname2)
	fmt.Println(ps2.ListPeers(topicname2))
}


var _ pubsub.SubscriptionFilter = (*Filter)(nil)
type Filter struct {
	allowedIDs map[peer.ID]struct{}
	topic string
}

func (filter *Filter) CanSubscribe(topic string) bool {
	return true
}

func (filter *Filter) FilterIncomingSubscriptions(from peer.ID, opts []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error) {
	if _, found := filter.allowedIDs[from]; !found {
		var newopts []*pb.RPC_SubOpts
		for _, opt := range opts {
			if *opt.Topicid != filter.topic {
				newopts = append(newopts, opt)
			} else {
				return nil, fmt.Errorf(">>>>>> message received on a topic on which peer %s should not publish", from.String())
			}
		}
		return newopts, nil
	}
	return opts, nil
}
