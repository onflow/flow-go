# Transit Bootstrap scripts

The transit script facilitates nodes uploading their public keys to dapper servers.

It also then handles securely recieving their RB Keys and network metadata after bootstrap data is created.

## Server token

The server token is needed with the `-t` flag for both commands. It authenticates the script to the server so that only trusted parties with the token may upload their node info and be included in the bootstrap data.

## Usage

```shell
$ transit push -t ${server-token} -d ${bootstrap-dir} -r ${flow-role}
$ transit pull -t ${server-token} -d ${bootstrap-dir} -r ${flow-role}
```

## Push

Running `transit push` will perform the following actions:

1. Create a Transit Keypair with libsodium and write it to
   - `<id>.transit-key.pub`
   - `<id>.transit-key.priv`
1. Upload the node's public files to the server
   - `<id>.transit-key.pub`
   - `<id>.node-info.pub.json`

## Pull

After bootstrapping, running `transit pull` will:

1. Fetch the following files:

   - `dkg-data.pub.json`
   - `node-infos.pub.json`
   - `root-protocol-snapshot.json`
   - `execution-state [dir]`
   - `<id>.random-beacon.priv.json.enc`

1. Decrypt `<id>.random-beacon.priv.json.enc` using the transit keys
   - `<id>.random-beacon.priv.json`

## Wrapping Responses

The transit script also has `-wrap` for the other end of the connection. This function takes a private random-beacon key and wraps it with the corresponding transit key, which can then be sent back to the node.

```shell
$ transit wrap -i ${ID} -r ${flow-role}
```

The wrap function:

1. Takes in `<id>.random-beacon.priv.json` and produces
   - `<id>.random-beacon.priv.json.enc`
1. Uploads `<id>.random-beacon.priv.json.enc` to the server
