module github.com/onflow/flow-go

go 1.13

require (
	cloud.google.com/go/storage v1.10.0
	github.com/HdrHistogram/hdrhistogram-go v0.9.0 // indirect
	github.com/bsipos/thist v1.0.0
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/codahale/hdrhistogram v0.9.0 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/ethereum/go-ethereum v1.9.13
	github.com/fxamacker/cbor/v2 v2.2.1-0.20201006223149-25f67fca9803
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.2
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/ipfs/go-log v1.0.4
	github.com/jrick/bitset v1.0.0
	github.com/libp2p/go-addr-util v0.0.2
	github.com/libp2p/go-libp2p v0.13.0
	github.com/libp2p/go-libp2p-core v0.8.0
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-pubsub v0.4.1
	github.com/libp2p/go-libp2p-swarm v0.4.0
	github.com/libp2p/go-libp2p-transport-upgrader v0.4.0
	github.com/libp2p/go-tcp-transport v0.2.1
	github.com/m4ksio/wal v1.0.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/onflow/cadence v0.12.5
	github.com/onflow/flow-core-contracts/lib/go/contracts v0.7.1
	github.com/onflow/flow-go-sdk v0.14.2
	github.com/onflow/flow-go/crypto v0.12.0
	github.com/onflow/flow/protobuf/go/flow v0.1.8
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/rs/zerolog v1.19.0
	github.com/spf13/cobra v0.0.6
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/uber/jaeger-client-go v2.22.1+incompatible
	github.com/uber/jaeger-lib v2.3.0+incompatible // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible
	github.com/vmihailenco/msgpack/v4 v4.3.11
	go.uber.org/atomic v1.6.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6
	google.golang.org/api v0.31.0
	google.golang.org/grpc v1.31.1
	gotest.tools v2.2.0+incompatible
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

// temp fix for MacOS build. See comment https://github.com/ory/dockertest/issues/208#issuecomment-686820414
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6

replace github.com/onflow/flow-go/crypto => ./crypto
