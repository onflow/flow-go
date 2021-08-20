module github.com/onflow/flow-go/integration

go 1.15

require (
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.0.1 // indirect
	github.com/dapperlabs/testingdock v0.4.3-0.20200626075145-ea23fc16bb90
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/dgraph-io/ristretto v0.0.3 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/ethereum/go-ethereum v1.10.1 // indirect
	github.com/go-openapi/strfmt v0.20.1 // indirect
	github.com/go-test/deep v1.0.7 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/onflow/cadence v0.18.1-0.20210730161646-b891a21c51fd
	github.com/onflow/flow-core-contracts/lib/go/contracts v0.7.6
	github.com/onflow/flow-core-contracts/lib/go/templates v0.7.6
	github.com/onflow/flow-emulator v0.20.3
	github.com/onflow/flow-go v0.18.0 // replaced by version on-disk
	github.com/onflow/flow-go-sdk v0.21.0
	github.com/onflow/flow-go/crypto v0.18.0 // replaced by version on-disk
	github.com/onflow/flow/protobuf/go/flow v0.2.2
	github.com/plus3it/gorecurcopy v0.0.1
	github.com/prometheus/common v0.20.0 // indirect
	github.com/rs/zerolog v1.21.0
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.25.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/vmihailenco/msgpack/v4 v4.3.12 // indirect
	github.com/vmihailenco/tagparser v0.1.2 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210405174219-a39eb2f71cb9 // indirect
	google.golang.org/grpc v1.36.1
	gopkg.in/yaml.v2 v2.4.0
)

// temp fix for MacOS build. See comment https://github.com/ory/dockertest/issues/208#issuecomment-686820414
//replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6

replace github.com/onflow/flow-go => ../

replace github.com/onflow/flow-go/crypto => ../crypto
