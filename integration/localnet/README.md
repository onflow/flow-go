# Flow Local Instrumented Test Environment (FLITE)

FLITE is a tool for running a full version of the Flow blockchain.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of Contents

- [Bootstrapping](#bootstrapping)
  - [Configuration](#configuration)
  - [Profiling](#profiling)
- [Start the network](#start-the-network)
- [Stop the network](#stop-the-network)
- [Logs](#logs)
- [Metrics](#metrics)
- [Loader](#loader)
- [Playing with Localnet](#playing-with-localnet)
  - [Configure Flow CLI to work with localnet](#configure-flow-cli-to-work-with-localnet)
    - [Add localnet network](#add-localnet-network)
    - [Add service account address and private key](#add-service-account-address-and-private-key)
    - [Configure contract addresses for localnet](#configure-contract-addresses-for-localnet)
  - [Using Flow CLI to send transaction to localnet](#using-flow-cli-to-send-transaction-to-localnet)
    - [Creating new account on the localnet](#creating-new-account-on-the-localnet)
    - [Getting account information](#getting-account-information)
    - [Running a cadence script](#running-a-cadence-script)
    - [Moving tokens from the service account to another account](#moving-tokens-from-the-service-account-to-another-account)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Bootstrapping

Before running the Flow network it is necessary to run a bootstrapping process.
This generates keys for each of the nodes and a genesis block to build on.

Bootstrap a new network:

```sh
make init
```

### Configuration

Various properties of the local network can be configured when it is initialized.
All configuration is optional.

Specify the number of nodes for each role:

```sh
make -e COLLECTION=2 CONSENSUS=5 EXECUTION=3 VERIFICATION=2 ACCESS=2 init
```

Specify the number of collector clusters:

```sh
make -e NCLUSTERS=3 init
```

### Profiling

You can turn on automatic profiling for all nodes. Profiles are written every 2
minutes to `./profiler`.

```sh
make -e PROFILER=true init
```

## Start the network

This command will automatically build new Docker images from your latest code changes
and then start the test network:

```sh
make start
```

## Stop the network

The network needs to be stopped between each consecutive run to clear the chain state:

```sh
make stop
```

## Logs

You can view log output from all nodes:

```sh
make logs
```

## Metrics

You can view realtime metrics while the network is running:

- Prometheus: http://localhost:9090/
- Traces (Jaeger): http://localhost:16686/
- Grafana: http://localhost:3000/
  - Username: `admin`
  - Password: `admin`

Here's an example of a Prometheus query that filters by the `consensus` role:

```
avg(rate(consensus_finalized_blocks{role="consensus"}[2m]))
```

## Loader

Localnet can be loaded easily as well

```
make load
```

The command by default will load your localnet with 1 tps for 30s, then 10 tps for 30s, and finally 100 tps indefinitely.

More about the loader can be found in the loader module.

## Playing with Localnet

This section documents how can be localnet used for experimenting with the network.

### Configure Flow CLI to work with localnet
Follow documentation to [install](https://docs.onflow.org/flow-cli/install/) and [initialize](https://docs.onflow.org/flow-cli/initialize-configuration/) the Flow CLI.

#### Add localnet network
Modify Flow CLI configuration file and add `"localnet"` network, using access node address/port with values displayed by the localnet initialization step.
An example of the Flow CLI configuration modified for connecting to the localnet:
```
{
	"networks": {
		"localnet": "127.0.0.1:3569"
	}
}
```
You can test the connection to the localnet by for example querying service account address:
```
flow -n localnet accounts get f8d6e0586b0a20c7
```
#### Add service account address and private key
The service account private key is hardcoded for localnet and can be found in [unit test utility execution state](https://github.com/onflow/flow-go/blob/master/utils/unittest/execution_state.go#L22).

The service account address is derived from network ID (in this case `"flow-localnet"`) and the generated service account address is `"f8d6e0586b0a20c7"`.
> Note: you can also get the address via [`Chain interface ServiceAddress()`](https://github.com/onflow/flow-go/blob/f943eebd59cc14648b9e574904ea40112ee42a9e/model/flow/chain.go#L226) method.

Create new entry in the Flow CLI config `"accounts"` section for the localnet service account and add the service account address and private key using the [advanced format](https://docs.onflow.org/flow-cli/configuration/#advanced-format-1).

An example of the Flow CLI configuration with the service account added:

```
{
	"networks": {
		"localnet": "127.0.0.1:3569"
	},
	"accounts": {
		"localnet-service-account": {
			"address": "f8d6e0586b0a20c7",
			"key":{
				"type": "hex",
        "index": 0,
        "signatureAlgorithm": "ECDSA_P256",
        "hashAlgorithm": "SHA2_256",
        "privateKey": "8ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc43"
			}
		}
	}
}
```
to check if the address above really is a service account, query the service account address:
```
flow -n localnet accounts get f8d6e0586b0a20c7
```
and check that the output contains `Contract: 'FlowServiceAccount'`.
#### Configure contract addresses for localnet

When you send transaction via CLI, the cadence contract you provide to the CLI will likely need to reference other contracts deployed on the network. You can configure the CLI to substitute the contract addresses on different networks. For example, to run  [this contract](https://github.com/onflow/flow-core-contracts/blob/master/transactions/flowToken/transfer_tokens.cdc), you will need to import FungibleToken and FlowToken.

Add this block to your Flow CLI configuration file `"contracts"` section:
```
"FungibleToken": {
	"source": "cadence/contracts/FungibleToken.cdc",
	"aliases": {
		"localnet": "0xee82856bf20e2aa6",
		"emulator": "0xee82856bf20e2aa6",
		"testnet": "0x9a0766d93b6608b7"
	}
},
"FlowToken": {
	"source": "cadence/contracts/FlowToken.cdc",
	"aliases": {
		"localnet": "0x0ae53cb6e3f42a79",
		"emulator": "0x0ae53cb6e3f42a79",
		"testnet": "0x7e60df042a9c0868"
	}
}
```
>Note: The actual address values can also be found in Flow Documentation for [Fungible Token Contract](https://docs.onflow.org/core-contracts/fungible-token/) and [Flow Token Contract](https://docs.onflow.org/core-contracts/flow-token/).

### Using Flow CLI to send transaction to localnet

#### Creating new account on the localnet

Create keys for a new account:
```
flow keys generate -n localnet
```
Use the generated public key in the following command:
```
flow accounts create --key <GENERATED_PUBLIC_KEY> --signer localnet-service-account -n localnet
```
After the transaction is sealed the command should print the account address and balance.

#### Getting account information

To verify that the account created in the previous section exists, or to check the balance etc. you can run:
```
flow -n localnet accounts get <ACCOUNT_ADDRESS>
```
#### Running a cadence script

This script below reads the balance field of an account's FlowToken Balance.
Create a file (for example `my_script.cdc`) containing following cadence code:

```
import FungibleToken from 0xee82856bf20e2aa6
import FlowToken from 0x0ae53cb6e3f42a79

pub fun main(address: Address): UFix64 {
	let acct = getAccount(address)
	let vaultRef = acct.getCapability(/public/flowTokenBalance)!.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
		?? panic("Could not borrow Balance reference to the Vault")
	return vaultRef.balance
}
```
Run the script:
```
flow scripts execute -n localnet ~/my_script.cdc "<ACCOUNT_ADDRESS>"
```
> replace `<ACCOUNT_ADDRESS>` in the command above with an address that you created on your localnet.

The script should output the account balance of the specified account.

You can also execute simple script without creating files, by providing the script in the command, for example:
```
# flow scripts execute -n localnet <(echo """
pub fun main(address: Address): UFix64 {
    return getAccount(address).balance
}
""") "<ACCOUNT_ADDRESS>"
```
#### Moving tokens from the service account to another account

Create new cadence contract file from [this template.](https://github.com/onflow/flow-core-contracts/blob/master/transactions/flowToken/transfer_tokens.cdc)
Make sure that contract imports have values that match your cli config, following the CLI configuration chapter above it should look like:
```
import FungibleToken from "cadence/contracts/FungibleToken.cdc"
import FlowToken from "cadence/contracts/FlowToken.cdc"
```
Send the transaction with this contract to localnet:
```
flow transactions send transfer_tokens.cdc 9999.9 <ACCOUNT_ADDRESS> -n localnet --signer localnet-service-account
```
> replace `<ACCOUNT_ADDRESS>` in the command above with an address that you created on your localnet.

After the transaction is sealed, the account with `<ACCOUNT_ADDRESS>` should have the balance increased by 9999.9 tokens.

# admin tool
The admin tool is enabled by default in localnet for all node type except access node.

For instance, in order to use admin tool to change log level, first find the local port that maps to `9002` which is the admin tool address, if the local port is `3702`, then run:
```
curl localhost:3702/admin/run_command -H 'Content-Type: application/json' -d '{"commandName": "set-log-level", "data": "debug"}'
```

To find the local port after launching the localnet, run `docker ps -a`, and find the port mapping.
For instance, the following result of `docker ps -a ` shows `localnet-collection` maps 9002 port to localhost's 3702 port, so we could use 3702 port to connect to admin tool.
```
2e0621f7e592   localnet-access                   "/bin/app --nodeid=9…"   9 seconds ago    Up 8 seconds              0.0.0.0:3571->9000/tcp, :::3571->9000/tcp, 0.0.0.0:3572->9001/tcp, :::3572->9001/tcp                                                           localnet_access_2_1
fcd92116f902   localnet-collection               "/bin/app --nodeid=0…"   9 seconds ago    Up 8 seconds              0.0.0.0:3702->9002/tcp, :::3702->9002/tcp                                                                                                      localnet_collection_1_1
dd841d389e36   localnet-access                   "/bin/app --nodeid=a…"   10 seconds ago   Up 9 seconds              0.0.0.0:3569->9000/tcp, :::3569->9000/tcp, 0.0.0.0:3570->9001/tcp, :::3570->9001/tcp                                                           localnet_access_1_1
```
