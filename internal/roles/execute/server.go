package execute

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	svc "github.com/dapperlabs/bamboo-node/grpc/services/execute"
	"github.com/dapperlabs/bamboo-node/internal/services/execute/config"
)

type Server struct {
	gsrv *grpc.Server
	conf *config.Config
}

func NewServer(
	conf *config.Config,
	ctrl *Controller,
) (*Server, error) {
	gsrv := grpc.NewServer()

	svc.RegisterExecuteServiceServer(gsrv, ctrl)

	return &Server{
		gsrv: gsrv,
		conf: conf,
	}, nil
}

// Start starts the server.
func (s *Server) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.conf.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// run the server, exit on error
	log.Fatal(s.gsrv.Serve(lis))
}
