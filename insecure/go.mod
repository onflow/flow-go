module github.com/onflow/flow-go/insecure

go 1.16

require (
	github.com/golang/protobuf v1.5.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ipfs/go-datastore v0.6.0
	github.com/libp2p/go-libp2p v0.28.1
	github.com/libp2p/go-libp2p-pubsub v0.9.3
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/onflow/flow-go v0.31.1-0.20230718164039-e3411eff1e9d
	github.com/onflow/flow-go/crypto v0.24.9
	github.com/rs/zerolog v1.29.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/yhassanzadeh13/go-libp2p-pubsub v0.6.11-flow-expose-msg.0.20230703223453-544e2fe28a26
	go.uber.org/atomic v1.11.0
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.31.0
)

replace github.com/onflow/flow-go => ../
