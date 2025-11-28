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

## Collection nodes

The transit script has three commands applicable to collection nodes:

```shell
$ transit pull-clustering -t ${server-token} -d ${bootstrap-dir}
$ transit generate-cluster-block-vote -t ${server-token} -d ${bootstrap-dir}
$ transit push-cluster-block-vote -t ${server-token} -d ${bootstrap-dir} -v ${vote-file}
```

### Pull Clustering

Running `transit pull-clustering` will:

1. Fetch the collection cluster assignment for the upcoming spork and write it to `<bootstrap-dir>/public-root-information/root-clustering.json`

### Sign Cluster Root Block

After the root clustering has been fetched, running `transit generate-cluster-block-vote` will:

1. Create a signature over the root block of the collection node's assigned cluster, using the node's private staking key.
2. Store the resulting vote to the file `<bootstrap-dir>/private-root-information/private-node-info_<node_id>/root-cluster-block-vote.json`

### Upload Vote

Once a vote has been generated, running `transit push-cluster-block-vote` will upload the vote file to the server.

## Root Block Signing Automation

The root block voting is a critical and time-sensitive step of a network upgrade (spork).
During this process, Collection Node operators must download the cluster assignment, generate votes from each of their Collection Nodes, and upload those votes to the server (one vote per node).
Then Consensus Node operators must download the root block, generate a vote for it using each of their Consensus Nodes, and upload those votes to the server (one vote per node).

To ensure this step is completed reliably and without delays, operators running multiple Collection Nodes or Consensus Nodes are strongly encouraged to automate the root block voting process.
The provided `vote_on_cluster_block.yml` and `vote_on_root_block.yml` Ansible playbooks serve as an example for building such automation. They automate:

- Pulling the root information
  - For Collection Nodes: the collection cluster assignment
  - For Consensus Nodes: the root block and random beacon key
- Generating the vote
- Pushing the vote to the server

for Collection and Consensus nodes respectively.

Refer to this playbook as a reference for how to use the transit script in an automated environment.

Example usage:

```shell
ansible-playbook -i inventories/mainnet/mainnet26.yml/ vote_on_root_block.yml \ 
    -e "boot_tools_tar=https://storage.googleapis.com/flow-genesis-bootstrap/boot-tools.tar" \
    -e "bootstrap_directory=/var/flow/bootstrap"
    -e "genesis_bucket=flow-genesis-bootstrap" \
    -e "network_version_token=mainnet-26" \
    -e "output_directory=/var/flow/bootstrap" \
    -e force_repull_transit=true \ 
```

