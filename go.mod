module github.com/dapperlabs/bamboo-node

go 1.12

require (
	github.com/FactomProject/basen v0.0.0-20150613233007-fe3947df716e // indirect
	github.com/FactomProject/btcutilecc v0.0.0-20130527213604-d3a63a5752ec // indirect
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/aristanetworks/goarista v0.0.0-20190628180533-8e7d5b18fe7a // indirect
	github.com/bluele/gcache v0.0.0-20190518031135-bc40bd653833
	github.com/btcsuite/btcd v0.0.0-20190629003639-c26ffa870fd8 // indirect
	github.com/cmars/basen v0.0.0-20150613233007-fe3947df716e // indirect
	github.com/dapperlabs/bamboo-node/language/runtime v0.0.0-00010101000000-000000000000
	github.com/ethereum/go-ethereum v1.8.27 // indirect
	github.com/go-pg/migrations v6.7.3+incompatible
	github.com/go-pg/pg v8.0.4+incompatible
	github.com/golang/protobuf v1.3.2
	github.com/google/wire v0.3.0
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/miguelmota/go-ethereum-hdwallet v0.0.0-20190601230056-2da794f11e15
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0
	github.com/psiemens/sconfig v0.0.0-20190623041652-6e01eb1354fc
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.5
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tyler-smith/go-bip32 v0.0.0-20170922074101-2c9cfd177564
	github.com/tyler-smith/go-bip39 v1.0.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/grpc v1.21.1
	mellium.im/sasl v0.2.1 // indirect
)

replace github.com/dapperlabs/bamboo-node/language/runtime => ./language/runtime
