# Transit Bootstrap scripts

The transit script is an utility used by node operators to upload and download relevant data before and after a Flow spork.
It is used to download the root snapshot after a spork.
Additionally, for a consensus node, it is used to upload transit keys and to submit root block votes.

## Server token

The server token is needed with the `-t` flag for all commands. It authenticates the script to the server so that only trusted parties with the token may upload their node info and be included in the bootstrap data.

## Usage

```shell
$ transit pull -t ${server-token} -d ${bootstrap-dir} -r ${flow-role}
```

### Pull

After bootstrapping, running `transit pull` will:

1. Fetch the following files:

   - `root-block.json` 
   - `node-infos.pub.json`
   - `root-protocol-snapshot.json`
   - `root-checkpoint` (only for execution nodes)
   - `random-beacon.priv.json.<id>.enc` (only for consensus nodes)

1. Decrypt `random-beacon.priv.json.<id>.enc` using the transit keys (only for consensus nodes)
   - `random-beacon.priv.json`

### Wrapping Responses

The transit script also has `wrap` for the other end of the connection. This function takes a private random-beacon key and wraps it with the corresponding transit key, which can then be sent back to the node.

```shell
$ transit wrap -i ${ID} -r ${flow-role}
```

The wrap function:

1. Takes in `random-beacon.priv.json` and produces
   - `random-beacon.priv.json.<id>.enc`
1. Uploads `random-beacon.priv.json.<id>.enc` to the server

## Consensus nodes

The transit script has four commands applicable to consensus nodes:

```shell
$ transit pull-root-block -t ${server-token} -d ${bootstrap-dir}
$ transit generate-root-block-vote -t ${server-token} -d ${bootstrap-dir}
$ transit push-root-block-vote -t ${server-token} -d ${bootstrap-dir} -v ${vote-file}
$ transit push-transit-keys -t ${server-token} -d ${bootstrap-dir}
```

### Pull Root Block and Random Beacon Key

Running `transit pull-root-block` will perform the following actions:

1. Fetch the root block for the upcoming spork and write it to `<bootstrap-dir>/public-root-information/root-block.json`
2. Fetch the random beacon key `random-beacon.priv.json.<id>.enc` and decrypt it using the transit keys

### Sign Root Block

After the root block and random beacon key have been fetched, running `transit generate-root-block-vote` will:

1. Create a combined signature over the root block using the node's private staking key and private random beacon key.
2. Store the resulting vote to the file `<bootstrap-dir>/private-root-information/private-node-info_<node_id>/root-block-vote.json`

### Upload Vote

Once a vote has been generated, running `transit push-root-block-vote` will upload the vote file to the server.

### Push Transit Key

Transit key is used to encrypt the random beacon key generated for the consensus nodes.

Running `transit push-transit-key` will perform the following actions:

1. Create a Transit Keypair and write it to
   - `transit-key.pub.<id>`
   - `transit-key.priv.<id>`
1. Upload the node's public files to the server
   - `transit-key.pub.<id>`
