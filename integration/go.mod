module github.com/onflow/flow-go/integration

go 1.13

require (
	github.com/dapperlabs/testingdock v0.4.3-0.20200626075145-ea23fc16bb90
	github.com/dgraph-io/badger/v2 v2.0.3
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	// pinned to https://github.com/onflow/cadence/commit/3ab62fe974547aba6fa4e8e102d16c3f0f4ecdf3
	// replace with tag before merging feature branch to master
	github.com/onflow/cadence v0.10.3
	github.com/onflow/flow-go v0.4.1-0.20200715183900-b337e998d486
	// pinned to https://github.com/onflow/flow-go-sdk/tree/janez/storage-fees
	// replace with tag before merging feature branch to master
	github.com/onflow/flow-go-sdk v0.12.1
	github.com/onflow/flow-go/crypto v0.9.4
	github.com/onflow/flow/protobuf/go/flow v0.1.8
	github.com/plus3it/gorecurcopy v0.0.1
	github.com/rs/zerolog v1.19.0
	github.com/stretchr/testify v1.6.1
	google.golang.org/grpc v1.31.1
	gopkg.in/yaml.v2 v2.2.8
)

// temp fix for MacOS build. See comment https://github.com/ory/dockertest/issues/208#issuecomment-686820414
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6

replace github.com/onflow/flow-go => ../

replace github.com/onflow/flow-go/crypto => ../crypto

replace github.com/onflow/flow-go/integration => ../integration
