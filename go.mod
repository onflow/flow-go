module github.com/dapperlabs/flow-go

go 1.13

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

replace github.com/dapperlabs/flow-go => ./

replace github.com/dapperlabs/flow-go/crypto => ./crypto

replace github.com/dapperlabs/flow-go/protobuf => ./protobuf

replace github.com/dapperlabs/flow-go/integration => ./integration

require (
	cloud.google.com/go/storage v1.9.0
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/dapperlabs/flow-core-contracts/contracts v0.0.0-20200526041238-ad2360621a1a
	github.com/dapperlabs/flow-go/crypto v0.0.0-00010101000000-000000000000
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/ethereum/go-ethereum v1.9.15
	github.com/go-kit/kit v0.8.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.4.2
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-log v1.0.4
	github.com/jrick/bitset v1.0.0
	github.com/libp2p/go-libp2p v0.10.0
	github.com/libp2p/go-libp2p-core v0.6.0
	github.com/libp2p/go-libp2p-pubsub v0.3.2
	github.com/libp2p/go-libp2p-swarm v0.2.7
	github.com/libp2p/go-libp2p-transport-upgrader v0.3.0
	github.com/libp2p/go-tcp-transport v0.2.0
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/onflow/cadence v0.4.0
	github.com/onflow/flow/protobuf/go/flow v0.1.4
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v0.9.1
	github.com/prometheus/tsdb v0.6.2-0.20190402121629-4f204dcbc150
	github.com/rs/zerolog v1.19.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.3.2
	github.com/stretchr/testify v1.6.1
	github.com/uber/jaeger-client-go v2.24.0+incompatible
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.12
	github.com/whyrusleeping/go-logging v0.0.1
	go.uber.org/atomic v1.6.0
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9
	google.golang.org/api v0.27.0
	google.golang.org/grpc v1.29.1
)
