package access

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/internals"
	"github.com/dapperlabs/bamboo-node/internal/nodes/access/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/access/controllers"
	"github.com/dapperlabs/bamboo-node/internal/nodes/access/data"
)

// Server ..
type Server struct {
	gsrv *grpc.Server
	conf *config.Config
}

// NewServer ..
func NewServer(
	dal *data.DAL,
	conf *config.Config,
	ctrl *controllers.Controller,
) (*Server, error) {

	err := dal.MigrateUp()
	if err != nil {
		return nil, err
	}

	gsrv := grpc.NewServer()
	bambooProto.RegisterAccessNodeServer(gsrv, ctrl)

	return &Server{
		gsrv: gsrv,
		conf: conf,
	}, nil
}

// Start starts the server
func (s *Server) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.conf.AppPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Run the server. Exit if error.
	log.Fatal(s.gsrv.Serve(lis))
}
