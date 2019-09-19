package collect

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	svc "github.com/dapperlabs/flow-go/pkg/grpc/services/collect"

	"github.com/dapperlabs/flow-go/internal/roles/collect/controller"
)

type Server struct {
	gsrv *grpc.Server
	conf *Config
	log  *logrus.Entry
}

func NewServer(
	conf *Config,
	log *logrus.Logger,
	ctrl *controller.Controller,
) (*Server, error) {
	gsrv := grpc.NewServer()

	svc.RegisterCollectServiceServer(gsrv, ctrl)

	return &Server{
		gsrv: gsrv,
		conf: conf,
		log:  logrus.NewEntry(log),
	}, nil
}

// Start starts the server.
func (s *Server) Start() {
	s.log.WithField("port", s.conf.Port).Info("Starting server...")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.conf.Port))
	if err != nil {
		s.log.WithError(err).Fatal("Failed to listen")
	}

	// run the server, exit on error
	err = s.gsrv.Serve(lis)
	if err != nil {
		s.log.WithError(err).Fatal("Failed to serve")
	}
}
