module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/flow-go v0.4.1-0.20200703230624-6b64cc19d38b
	github.com/dapperlabs/flow-go/crypto v0.3.2-0.20200312195452-df4550a863b7
	github.com/dapperlabs/testingdock v0.4.3-0.20200626075145-ea23fc16bb90
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/onflow/cadence v0.5.1
	github.com/onflow/flow-go-sdk v0.5.0
	github.com/onflow/flow/protobuf/go/flow v0.1.5-0.20200611205353-548107cc9aca
	github.com/plus3it/gorecurcopy v0.0.1
	github.com/rs/zerolog v1.19.0
	github.com/stretchr/testify v1.6.1
	google.golang.org/grpc v1.28.0
	gopkg.in/yaml.v2 v2.2.5
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/protobuf => ../protobuf

replace github.com/dapperlabs/flow-go/crypto => ../crypto

replace github.com/dapperlabs/flow-go/integration => ../integration
