# Bamboo Emulator CLI

## Usage

opens up `bamboo` in interactive mode
```bash
> bamboo
```
available commands:
 - emulator
 - client
 - help
 - exit


starts up emulator server (non-interactive) and logs emulator activity
```bash
> bamboo emulator start [-p PORT] [-v VERBOSE]
```


opens up `bamboo` client in interactive mode
```bash
> bamboo client
```
requires that emulator server running in the background (in a different terminal)
available commands:
 - account
 - query
 - help
 - exit


## Example
### Emulator Server
```bash
> bamboo emulator start -v -p 1234
INFO[0000] Starting emulator server...                   port=1234
INFO[0000] Generating wallet from mneumonic: primary chuckle hazard injury swamp exact depth mad hurdle junior clarify seminar refuse keep vapor piece differ dream begin ethics swim measure trap decrease  mnemonic="primary chuckle hazard injury swamp exact depth mad hurdle junior clarify seminar refuse keep vapor piece differ dream begin ethics swim measure trap decrease"
INFO[0000] Creating root account 0x6726f70045f4a11064ba97509de88950f4c72ea0 from derivation path m/44'/60'/0'/0/0  address=0x6726f70045f4a11064ba97509de88950f4c72ea0 balance=100 path="m/44'/60'/0'/0/0"
INFO[0000] Minting genesis block (0x229109c19f4d618a417fe66f22917ed734a49eabdbac141bac9dbe66db486659)  blockHash=229109c19f4d618a417fe66f22917ed734a49eabdbac141bac9dbe66db486659 blockNum=0 numCollections=0 numTransactions=0
...
```

### Emulator Client
```bash
> bamboo client

Usage:
    bamboo% [command] [args]

Available Commands:
    account     Account operations.
    query       Query operations.
    help        Prints help tool.
    exit        Exits the client.

bamboo% 
```