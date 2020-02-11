module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/flow-go v0.3.1
	github.com/dapperlabs/flow-go-sdk v0.3.0
	github.com/dapperlabs/flow-go/language v0.3.1
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/m4ksio/testingdock v0.4.1
	github.com/stretchr/testify v1.4.0
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/integration => ./

replace github.com/dapperlabs/flow-go/crypto => ../crypto
