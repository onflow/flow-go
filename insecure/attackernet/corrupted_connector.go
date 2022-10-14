package attackernet

import (
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/logging"
)

type CorruptConnector struct {
	logger         zerolog.Logger
	inboundHandler func(*insecure.Message)
	corruptNodeIds flow.IdentityList // identifier of the corrupt nodes

	// ports on which each corrupt node's corrupt network is running.
	// each corrupt node is running a gRPC server in a docker container, while the orchestrator network is running a gRPC client on local host.
	// hence, each container comes with a port binding on local host.
	corruptPortsMap map[flow.Identifier]string
}

var _ insecure.CorruptedNodeConnector = &CorruptConnector{}

func NewCorruptConnector(
	logger zerolog.Logger,
	corruptNodeIds flow.IdentityList,
	corruptPortsMap map[flow.Identifier]string) *CorruptConnector {
	return &CorruptConnector{
		logger:          logger.With().Str("component", "corrupt-connector").Logger(),
		corruptNodeIds:  corruptNodeIds,
		corruptPortsMap: corruptPortsMap,
	}
}

// Connect creates a connection the corrupt network of the given corrupt identity.
func (c *CorruptConnector) Connect(ctx irrecoverable.SignalerContext, targetId flow.Identifier) (insecure.CorruptedNodeConnection, error) {
	if c.inboundHandler == nil {
		return nil, fmt.Errorf("inbound handler has not set")
	}

	port, ok := c.corruptPortsMap[targetId]
	if !ok {
		return nil, fmt.Errorf("could not find port mapping for corrupt id: %x", targetId)
	}

	// corrupt nodes are running on docker containers, while the orchestrator network is on local host.
	// hence, each container is accessible on local host through port binding.
	corruptAddress := fmt.Sprintf("localhost:%s", port)
	gRpcClient, err := grpc.Dial(
		corruptAddress,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not dial corrupt network %s: %w", corruptAddress, err)
	}

	client := insecure.NewCorruptNetworkClient(gRpcClient)

	inbound, err := client.ConnectAttacker(ctx, &empty.Empty{})
	if err != nil {
		return nil, fmt.Errorf("could not establish an inbound stream to corrupt network: %w", err)
	}

	outbound, err := client.ProcessAttackerMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not establish an outbound stream to corrupt network: %w", err)
	}

	connection := NewCorruptedNodeConnection(c.logger, c.inboundHandler, outbound, inbound)
	connection.Start(ctx)

	c.logger.Debug().
		Hex("target_id", logging.ID(targetId)).
		Msg("starting corrupt connector")

	<-connection.Ready()

	c.logger.Info().
		Hex("target_id", logging.ID(targetId)).
		Msg("corrupt connection started and established")

	return connection, nil
}

// WithIncomingMessageHandler sets the handler for the incoming messages from remote corrupt nodes.
func (c *CorruptConnector) WithIncomingMessageHandler(handler func(*insecure.Message)) {
	c.inboundHandler = handler
}
