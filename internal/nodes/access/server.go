package access

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	collectSvc "github.com/dapperlabs/bamboo-node/grpc/services/collect"
	observeSvc "github.com/dapperlabs/bamboo-node/grpc/services/observe"
	pingSvc "github.com/dapperlabs/bamboo-node/grpc/services/ping"
	verifySvc "github.com/dapperlabs/bamboo-node/grpc/services/verify"
	"github.com/dapperlabs/bamboo-node/internal/nodes/access/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/ping"
	"github.com/dapperlabs/bamboo-node/internal/protocol/collect"
	"github.com/dapperlabs/bamboo-node/internal/protocol/observe"
	"github.com/dapperlabs/bamboo-node/internal/protocol/verify"
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
	observeCtrl *observe.Controller,
	collectCtrl *collect.Controller,
	verifyCtrl *verify.Controller,
) (*Server, error) {
	gsrv := grpc.NewServer()

	pingSvc.RegisterPingServiceServer(gsrv, pingCtrl)
	observeSvc.RegisterObserveServiceServer(gsrv, observeCtrl)
	collectSvc.RegisterCollectServiceServer(gsrv, collectCtrl)
	verifySvc.RegisterVerifyServiceServer(gsrv, verifyCtrl)

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
