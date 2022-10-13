package orchestrator

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

type CorruptedConnector struct {
	logger           zerolog.Logger
	inboundHandler   func(*insecure.Message)
	corruptedNodeIds flow.IdentityList // identifier of the corrupted nodes

	// ports on which each corrupted node's conduit factory is running.
	// corrupted nodes are running on docker containers, while the orchestrator network is on local host.
	// hence, each container comes with a port binding on local host.
	corruptedPortMapping map[flow.Identifier]string
}

func NewCorruptedConnector(
	logger zerolog.Logger,
	corruptedNodeIds flow.IdentityList,
	corruptedPortMapping map[flow.Identifier]string) *CorruptedConnector {
	return &CorruptedConnector{
		logger:               logger.With().Str("component", "corrupted-connector").Logger(),
		corruptedNodeIds:     corruptedNodeIds,
		corruptedPortMapping: corruptedPortMapping,
	}
}

// Connect creates a connection the corruptible conduit factory of the given corrupted identity.
func (c *CorruptedConnector) Connect(ctx irrecoverable.SignalerContext, targetId flow.Identifier) (insecure.CorruptedNodeConnection, error) {
	if c.inboundHandler == nil {
		return nil, fmt.Errorf("inbound handler has not set")
	}

	port, ok := c.corruptedPortMapping[targetId]
	if !ok {
		return nil, fmt.Errorf("could not find port mapping for corrupted id: %x", targetId)
	}

	// corrupted nodes are running on docker containers, while the orchestrator network is on local host.
	// hence, each container is accessible on local host through port binding.
	corruptedAddress := fmt.Sprintf("localhost:%s", port)
	gRpcClient, err := grpc.Dial(
		corruptedAddress,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not dial corruptible conduit factory %s: %w", corruptedAddress, err)
	}

	client := insecure.NewCorruptNetworkClient(gRpcClient)

	inbound, err := client.ConnectAttacker(ctx, &empty.Empty{})
	if err != nil {
		return nil, fmt.Errorf("could not establish an inbound stream to corruptible conduit factory: %w", err)
	}

	outbound, err := client.ProcessAttackerMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not establish an outbound stream to corruptible conduit factory: %w", err)
	}

	connection := NewCorruptedNodeConnection(c.logger, c.inboundHandler, outbound, inbound)
	connection.Start(ctx)

	c.logger.Debug().
		Hex("target_id", logging.ID(targetId)).
		Msg("starting a corrupted connector")

	<-connection.Ready()

	c.logger.Info().
		Hex("target_id", logging.ID(targetId)).
		Msg("corrupted connection started and established")

	return connection, nil
}

// WithIncomingMessageHandler sets the handler for the incoming messages from remote corrupted nodes.
func (c *CorruptedConnector) WithIncomingMessageHandler(handler func(*insecure.Message)) {
	c.inboundHandler = handler
}
