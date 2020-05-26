module github.com/dapperlabs/flow-go

go 1.13

require (
	cloud.google.com/go/storage v1.6.0
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/dapperlabs/flow-core-contracts/contracts v0.0.0-20200526041238-ad2360621a1a
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200312195452-df4550a863b7
	github.com/dgraph-io/badger/v2 v2.0.2
	github.com/ethereum/go-ethereum v1.9.9
	github.com/go-kit/kit v0.9.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-log v0.0.1
	github.com/jrick/bitset v1.0.0
	github.com/libp2p/go-libp2p v0.5.1
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/libp2p/go-libp2p-pubsub v0.2.7-0.20200226020716-1f147c24576a
	github.com/libp2p/go-libp2p-swarm v0.2.2
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/onflow/cadence v0.4.0-beta1.0.20200528102553-06110caee6bf
	github.com/onflow/flow-go-sdk v0.4.0-beta1.0.20200602171133-99bba437c939 // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.3.0
	github.com/prometheus/tsdb v0.7.1
	github.com/rs/zerolog v1.15.0
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.5.1
	github.com/uber/jaeger-client-go v2.22.1+incompatible
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.11
	github.com/whyrusleeping/go-logging v0.0.1
	go.uber.org/atomic v1.5.1
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6
	google.golang.org/api v0.18.0
	google.golang.org/grpc v1.28.0
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

replace github.com/dapperlabs/flow-go => ./

replace github.com/dapperlabs/flow-go/crypto => ./crypto

replace github.com/dapperlabs/flow-go/protobuf => ./protobuf

replace github.com/dapperlabs/flow-go/integration => ./integration
