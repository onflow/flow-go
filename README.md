# bamboo-emulator

## Getting Started

### Bamboo Emulator CLI
```bash
> go get github.com/dapperlabs/bamboo-emulator
> bamboo-emulator
Usage:
  bamboo [command]

Available Commands:
  client      Bamboo Client operations
  emulator    Bamboo Emulator Server operations
  help        Help about any command
  init        Initialize new and empty Bamboo project

Flags:
  -h, --help   help for bamboo

Use "bamboo [command] --help" for more information about a command.
> bamboo-emulator init
INFO[0000] Bamboo Client setup finished! Begin by running: bamboo emulator start
> bamboo-emulator emulator start
INFO[0000] Starting emulator server...                   port=5000
INFO[0000] Generating wallet from mneumonic: test evil balcony skin behave rookie smile adult knee cushion keen robot chair defense item myth purpose stereo immune easily inch pitch icon split  mnemonic="test evil balcony skin behave rookie smile adult knee cushion keen robot chair defense item myth purpose stereo immune easily inch pitch icon split"
INFO[0000] Minting genesis block (0x83bb01b0106bd1e7c401a992e1bf7e5af8f041a26bf1e80ab38d21313693235b)  blockHash=83bb01b0106bd1e7c401a992e1bf7e5af8f041a26bf1e80ab38d21313693235b blockNum=0 numCollections=0 numTransactions=0
...
```

### Docker (outdated)
TODO: update this
```bash
> docker build -t bamboo-emulator .
> docker run -p 5000:5000 -it bamboo-emulator
```

### Submitting Transactions via gRPC
TODO: update this
```bash
> prototool grpc services --address 0.0.0.0:5000 --method bamboo.services.access.v1.BambooAccessAPI/SendTransaction --data '{"transaction":{ "nonce": 1}}'
```