package collect

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/config"
	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"
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

	svc.RegisterCollectServiceServer(gsrv, ctrl)

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
