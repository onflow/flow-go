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
INFO[0000] Loading mnemonic file from: bamboo.mnemonic   filename=bamboo.mnemonic
INFO[0000] Generating wallet from mneumonic: test evil balcony skin behave rookie smile adult knee cushion keen robot chair defense item myth purpose stereo immune easily inch pitch icon split  mnemonic="test evil balcony skin behave rookie smile adult knee cushion keen robot chair defense item myth purpose stereo immune easily inch pitch icon split"
INFO[0000] Creating root account 0xe39414f0d8aa7f9becccaab0569de8ee8dc9b644 from derivation path m/44'/60'/0'/0/0  address=0xe39414f0d8aa7f9becccaab0569de8ee8dc9b644 balance=100 path="m/44'/60'/0'/0/0"
INFO[0000] Minting genesis block (0x54d515bf6f430d51ffebdbc6a962ed890b8b40ffed1cc1468b89d069c3435b2f)  blockHash=54d515bf6f430d51ffebdbc6a962ed890b8b40ffed1cc1468b89d069c3435b2f blockNum=0 numCollections=0 numTransactions=0
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