package security

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	consensusSvc "github.com/dapperlabs/bamboo-node/grpc/services/consensus"
	pingSvc "github.com/dapperlabs/bamboo-node/grpc/services/ping"
	sealSvc "github.com/dapperlabs/bamboo-node/grpc/services/seal"

	"github.com/dapperlabs/bamboo-node/internal/nodes/ping"
	"github.com/dapperlabs/bamboo-node/internal/nodes/security/config"
	"github.com/dapperlabs/bamboo-node/internal/protocol/consensus"
	"github.com/dapperlabs/bamboo-node/internal/protocol/seal"
)

// Server ..
type Server struct {
	gsrv *grpc.Server
	conf *config.Config
}

// NewServer ..
func NewServer(
	conf *config.Config,
	pingCtrl *ping.Controller,
	consensusCtrl *consensus.Controller,
	sealCtrl *seal.Controller,
) (*Server, error) {
	gsrv := grpc.NewServer()

	pingSvc.RegisterPingServiceServer(gsrv, pingCtrl)
	consensusSvc.RegisterConsensusServiceServer(gsrv, consensusCtrl)
	sealSvc.RegisterSealServiceServer(gsrv, sealCtrl)

	return &Server{
		gsrv: gsrv,
		conf: conf,
	}, nil
}

// Start starts the server
func (s *Server) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.conf.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Run the server. Exit if error.
	log.Fatal(s.gsrv.Serve(lis))
}
