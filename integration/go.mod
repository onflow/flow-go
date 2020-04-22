module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/cadence v0.0.0-20200327205214-136b868762e2
	github.com/dapperlabs/flow-go v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/testingdock v0.4.2
	github.com/dgraph-io/badger/v2 v2.0.2
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/onflow/flow/protobuf/go/flow v0.1.3
	github.com/rs/zerolog v1.15.0
	github.com/stretchr/testify v1.5.1
	google.golang.org/grpc v1.28.0
	gotest.tools v2.2.0+incompatible
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf

replace github.com/dapperlabs/flow-go/crypto => ../crypto
