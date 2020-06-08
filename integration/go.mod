module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/flow-go v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/testingdock v0.4.3-0.20200424073047-26b38aa03602
	github.com/dgraph-io/badger/v2 v2.0.2
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/ethereum/go-ethereum v1.9.9
	github.com/onflow/cadence v0.4.0
	github.com/onflow/flow-go-sdk v0.4.0
	github.com/onflow/flow/protobuf/go/flow v0.1.5-0.20200601215056-34a11def1d6b
	github.com/rs/zerolog v1.15.0
	github.com/stretchr/testify v1.5.1
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	golang.org/x/exp v0.0.0-20200224162631-6cc2880d07d6
	google.golang.org/grpc v1.28.0
	gopkg.in/yaml.v2 v2.2.4
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf

replace github.com/dapperlabs/flow-go/crypto => ../crypto

replace github.com/dapperlabs/flow-go/integration => ../integration
