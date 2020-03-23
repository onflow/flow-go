module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/flow-go v0.3.2-0.20200225001519-9a07bfb642cb
	github.com/dapperlabs/flow-go-sdk v0.4.1
	github.com/dgraph-io/badger/v2 v2.0.2
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/m4ksio/testingdock v0.4.1
	github.com/stretchr/testify v1.5.1
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/crypto => ../crypto
