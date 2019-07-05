package execute

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	executeSvc "github.com/dapperlabs/bamboo-node/grpc/services/execute"
	pingSvc "github.com/dapperlabs/bamboo-node/grpc/services/ping"
	"github.com/dapperlabs/bamboo-node/internal/nodes/execute/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/ping"
	"github.com/dapperlabs/bamboo-node/internal/protocol/execute"
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
	executeCtrl *execute.Controller,
) (*Server, error) {
	gsrv := grpc.NewServer()

	pingSvc.RegisterPingServiceServer(gsrv, pingCtrl)
	executeSvc.RegisterExecuteServiceServer(gsrv, executeCtrl)

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
