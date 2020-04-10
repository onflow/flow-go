module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/cadence v0.0.0-20200401151421-7c78a2da20a6
	github.com/dapperlabs/flow-go v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/flow/protobuf/go/flow v0.0.0-20200405045545-03db81265125
	github.com/dapperlabs/testingdock v0.4.2
	github.com/dgraph-io/badger/v2 v2.0.2
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/stretchr/testify v1.5.1
	google.golang.org/grpc v1.28.0
	gotest.tools v2.2.0+incompatible
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf

replace github.com/dapperlabs/flow-go/crypto => ../crypto
