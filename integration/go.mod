module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/flow-go v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/testingdock v0.4.3-0.20200424073047-26b38aa03602
	github.com/dgraph-io/badger/v2 v2.0.2
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/onflow/cadence v0.1.0
	github.com/onflow/flow-go-sdk v0.1.0
	github.com/onflow/flow/protobuf/go/flow v0.1.4-0.20200424184233-4767141296a0
	github.com/rs/zerolog v1.15.0
	github.com/stretchr/testify v1.5.1
	google.golang.org/grpc v1.28.0
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf

replace github.com/dapperlabs/flow-go/crypto => ../crypto
