# Transit Bootstrap scripts
The transit script facilitates nodes uploading their public keys to dapper servers.

It also then handles securely recieving their RB Keys and network metadata after genesis is created.

## Server token
The server token is needed with the `-t` flag for both commands. It authenticates the script to the server so that only trusted parties with the token may upload their node info and be included in Genesis.

## Usage
```bash
$ ./transit -push -t ${server-token} -d ${bootstrap-dir}
$ ./transit -pull -t ${server-token} -d ${bootstrap-dir}
```

## Push
Running `-push` will perform the following actions:

1. Create a Transit Keypair with libsodium and write it to 
   - `<id>.transit-key.pub`
   - `<id>.transit-key.priv`
1. Upload the node's public files to the server
   - `<id>.transit-key.pub`
   - `<id>.node-info.pub.json`

## Pull
After Genesis, rulling `-pull` will:

1. Fetch the following files:
   - `dkg-data.pub.json`
   - `node-infos.pub.json`
   - `genesis-block.json`
   - `execution-state [dir]`
   - `<id>.random-beacon.priv.json.enc`

1. Decrypt `<id>.random-beacon.priv.json.enc` using the transit keys
   - `<id>.random-beacon.priv.json`