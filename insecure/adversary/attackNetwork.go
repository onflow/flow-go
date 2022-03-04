package adversary

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network"
)

type corruptedNodeConnection insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient

type AttackNetwork struct {
	component.Component
	corruptedIds   flow.IdentityList
	corruptedNodes map[flow.Identifier]corruptedNodeConnection
	codec          network.Codec
}

func NewAttackNetwork(corruptedIds flow.IdentityList) *AttackNetwork {
	attackNetwork := &AttackNetwork{
		corruptedIds:   corruptedIds,
		corruptedNodes: make(map[flow.Identifier]corruptedNodeConnection),
	}

	return attackNetwork
}

func (a *AttackNetwork) RpcUnicastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetId flow.Identifier) error {
	connection, ok := a.corruptedNodes[corruptedId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", corruptedId)
	}

	msg, err := a.eventToMessage(corruptedId, event, channel, insecure.Protocol_UNICAST, 0, targetId)
	if err != nil {
		return fmt.Errorf("could not convert event to unicast message: %w", err)
	}

	err = connection.Send(msg)
	if err != nil {
		return fmt.Errorf("could not sent unicast event to corrupted node: %w", err)
	}

	return nil
}

func (a AttackNetwork) RpcPublishOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetIds ...flow.Identifier) error {
	connection, ok := a.corruptedNodes[corruptedId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", corruptedId)
	}

	msg, err := a.eventToMessage(corruptedId, event, channel, insecure.Protocol_PUBLISH, 0, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to publish message: %w", err)
	}

	err = connection.Send(msg)
	if err != nil {
		return fmt.Errorf("could not sent publish event to corrupted node: %w", err)
	}

	return nil
}

func (a AttackNetwork) RpcMulticastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, num uint32, targetIds ...flow.Identifier) error {
	connection, ok := a.corruptedNodes[corruptedId]
	if !ok {
		return fmt.Errorf("no connection available for corrupted conduit factory to node %x: ", corruptedId)
	}

	msg, err := a.eventToMessage(corruptedId, event, channel, insecure.Protocol_MULTICAST, num, targetIds...)
	if err != nil {
		return fmt.Errorf("could not convert event to multicast message: %w", err)
	}

	err = connection.Send(msg)
	if err != nil {
		return fmt.Errorf("could not sent multicast event to corrupted node: %w", err)
	}

	return nil
}

func (a *AttackNetwork) start(ctx context.Context) error {
	for _, corruptedId := range a.corruptedIds {
		corruptibleClient, err := a.corruptibleConduitFactoryClient(ctx, corruptedId.Address)
		if err != nil {
			return fmt.Errorf("could not establish corruptible client to node %x: %w", corruptedId.NodeID, err)
		}
		a.corruptedNodes[corruptedId.NodeID] = corruptibleClient
	}

	return nil
}

// corruptedConduitFactoryAddress generates and returns the gRPC interface address of corruptible conduit factory for given identity.
func corruptedConduitFactoryAddress(address string) (string, error) {
	corruptedAddress, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("could not extract address of corruptible conduit factory %s: %w", address, err)
	}

	return net.JoinHostPort(corruptedAddress, strconv.Itoa(insecure.CorruptedFactoryPort)), nil
}

// corruptibleConduitFactoryClient creates a gRPC client for the corruptible conduit factory of the given corrupted identity. It then
// connects the client to the remote corruptible conduit factory and returns it.+
func (a *AttackNetwork) corruptibleConduitFactoryClient(ctx context.Context, address string) (corruptedNodeConnection, error) {
	corruptedAddress, err := corruptedConduitFactoryAddress(address)
	if err != nil {
		return nil, fmt.Errorf("could not generate corruptible conduit factory address for: %w", err)
	}
	gRpcClient, err := grpc.Dial(corruptedAddress)
	if err != nil {
		return nil, fmt.Errorf("could not dial corruptible conduit factory %s: %w", corruptedAddress, err)
	}

	client := insecure.NewCorruptibleConduitFactoryClient(gRpcClient)
	stream, err := client.ProcessAttackerMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not establish a stream to corruptible conduit factory: %w", err)
	}

	return stream, nil
}

// eventToMessage converts the given application layer event to a protobuf message that is meant to be sent to the corrupted node.
func (a *AttackNetwork) eventToMessage(corruptedId flow.Identifier,
	event interface{},
	channel network.Channel,
	protocol insecure.Protocol,
	num uint32,
	targetIds ...flow.Identifier) (*insecure.Message, error) {

	payload, err := a.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	return &insecure.Message{
		ChannelID: channel.String(),
		OriginID:  corruptedId[:],
		Targets:   num,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}, nil
}
