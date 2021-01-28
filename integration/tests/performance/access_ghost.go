package performance

import "google.golang.org/grpc"

type AccessGhost struct {

}

func NewAccessGhost() *AccessGhost {
	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
	}
}
