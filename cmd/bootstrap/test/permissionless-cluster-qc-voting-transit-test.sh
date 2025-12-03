#!/usr/bin/env bash

# This file implements a test for bootstrapping a network while having <2/3rds of private keys available for Collector nodes.
# In particular, it tests decentralized voting by Collection nodes on root cluster blocks as part of the bootstrapping process.
# The test can be run using either local or GCP bucket vote transport. To test GCP bucket transport, set the `bucket` and `token` variables below.
# To run this test, you must have `jq` and `gsutil` installed.

bucket=
token=

bootstrapcmd="go run .."
transitcmd="go run ../transit"

clusteringpath="public-root-information/root-clustering.json"
clustervotespath="public-root-information/root-block-votes"
# partner dir must end with `public-root-information`, otherwise partner nodeinfo will not be read from it
partner_dir="./public-root-information"
# keygen dir must not exist yet, or be empty
keygen_dir="./keygen"

clusterCount=2

# exit early if anything fails
set -e
# avoid overwriting existing files or using data from a previous run
if [ "$(ls | wc -l)" -gt 1 ]
then
  echo "Found files in $(pwd), please clean up after previous runs:"
  echo "rm -rf permissionless public-root-information private-root-information execution-state $keygen_dir $partner_dir node-config.json partner-weights.json"
  exit 1
fi

$bootstrapcmd genconfig --address-format "%s%d.example.com:3569" \
    --access 2 --collection 7 --consensus 3 --execution 2 --verification 1 --weight 100 \
    -o ./ --config ./node-config.json
$bootstrapcmd keygen --config ./node-config.json -o "$keygen_dir"


echo "simulating permissionless nodes"
# mark >33% of collectors permissionless / non-internal
permissionless_collectors=$(jq -r 'map(select(.["Role"]=="collection")) | .[:(length/3|floor)+1] | .[] | .["NodeID"]' \
    -- "$keygen_dir/public-root-information/node-internal-infos.pub.json")

# generate partner-weights file
jq 'map({(.["NodeID"]):.["Weight"]}) | add' \
    -- "$keygen_dir/public-root-information/node-internal-infos.pub.json" > ./partner-weights.json

# generate partner-node-infos (for non-internal nodes only)
mkdir -p "$partner_dir"
for node in $permissionless_collectors
do
  jq --arg jq_node_id "$node" '.[] | select(.["NodeID"]==$jq_node_id)' \
      -- "$keygen_dir/public-root-information/node-internal-infos.pub.json" \
      > "$partner_dir/node-info.pub.$node.json"
done

# create a directory for each permissionless node to store its private information
for node in $permissionless_collectors
do
  mkdir -p "./permissionless/$node/private-root-information"
  mkdir -p "./permissionless/$node/public-root-information"
  echo "$node" >> "./permissionless/$node/public-root-information/node-id"
  mv "$keygen_dir/private-root-information/private-node-info_$node" "./permissionless/$node/private-root-information/"
done


$bootstrapcmd cluster-assignment \
    --epoch-counter 0 \
    --collection-clusters $clusterCount \
    --clustering-random-seed 00000000000000000000000000000000000000000000000000000000deadbeef \
    --config ./node-config.json \
    -o ./ \
    --partner-dir "$partner_dir" \
    --partner-weights ./partner-weights.json \
    --internal-priv-dir "$keygen_dir/private-root-information"


#confirm that the bootstrapping process cannot continue yet (not enough votes for cluster QCs)
echo "Expecting FTL (not enough votes for Cluster QCs)..."
$bootstrapcmd rootblock \
    --root-chain bench \
    --root-height 0 \
    --root-parent 0000000000000000000000000000000000000000000000000000000000000000 \
    --root-view 0 \
    --epoch-counter 0 \
    --epoch-length 30000 \
    --epoch-staking-phase-length 20000 \
    --epoch-dkg-phase-length 2000 \
    --random-seed 00000000000000000000000000000000000000000000000000000000deadbeef \
    --collection-clusters $clusterCount \
    --use-default-epoch-timing \
    --kvstore-finalization-safety-threshold=1000 \
    --kvstore-epoch-extension-view-count=2000 \
    --config ./node-config.json \
    -o ./ \
    --partner-dir "$partner_dir" \
    --partner-weights ./partner-weights.json \
    --internal-priv-dir "$keygen_dir/private-root-information" \
    --intermediary-clustering-data "./$clusteringpath" \
    --cluster-votes-dir "./$clustervotespath" \
    | grep "not enough votes to create qc"

echo "Collecting votes for Cluster QCs..."
# permissionless collectors retrieve the clustering data
if [ -n "$bucket" ]
then
  # upload to cloud bucket; collectors pull using transit script
  echo "uploading to cloud bucket..."
  gsutil cp "./$clusteringpath" "gs://$bucket/$token/$clusteringpath"
  for node in $permissionless_collectors
  do
    $transitcmd pull-clustering -g "$bucket" -t "$token" -b "./permissionless/$node"
  done
else
  # copy locally
  for node in $permissionless_collectors
  do
    cp "./$clusteringpath" "./permissionless/$node/$clusteringpath"
  done
fi

# cluster voting
for node in $permissionless_collectors
do
  $transitcmd generate-cluster-block-vote -b "./permissionless/$node"
done

# collectors push votes, and bootstrapping machine retrieves them
if [ -n "$bucket" ]
then
  # collectors push using transit script
  for node in $permissionless_collectors
  do
    $transitcmd push-cluster-block-vote -g "$bucket" -t "$token" -b "./permissionless/$node"
  done
  gsutil cp "gs://$bucket/$token/root-cluster-block-vote.*" "./$clustervotespath/"
else
  # copy locally
  for node in $permissionless_collectors
  do
    cp "./permissionless/$node/private-root-information/private-node-info_$node/root-cluster-block-vote.json" "./$clustervotespath/root-cluster-block-vote.$node.json"
  done
fi


# root block creation should succeed now that we have enough votes
$bootstrapcmd rootblock \
    --root-chain bench \
    --root-height 0 \
    --root-parent 0000000000000000000000000000000000000000000000000000000000000000 \
    --root-view 0 \
    --epoch-counter 0 \
    --epoch-length 30000 \
    --epoch-staking-phase-length 20000 \
    --epoch-dkg-phase-length 2000 \
    --random-seed 00000000000000000000000000000000000000000000000000000000deadbeef \
    --collection-clusters $clusterCount \
    --use-default-epoch-timing \
    --kvstore-finalization-safety-threshold=1000 \
    --kvstore-epoch-extension-view-count=2000 \
    --config ./node-config.json \
    -o ./ \
    --partner-dir "$partner_dir" \
    --partner-weights ./partner-weights.json \
    --internal-priv-dir "$keygen_dir/private-root-information" \
    --intermediary-clustering-data "./$clusteringpath" \
    --cluster-votes-dir "./$clustervotespath"


# root block finalization should succeed with enough consensus votes
$bootstrapcmd finalize \
    --config ./node-config.json \
    --partner-dir "./$partner_dir" \
    --partner-weights ./partner-weights.json \
    --internal-priv-dir "$keygen_dir/private-root-information" \
    --dkg-data ./private-root-information/root-dkg-data.priv.json \
    --root-block ./public-root-information/root-block.json \
    --intermediary-bootstrapping-data ./public-root-information/intermediary-bootstrapping-data.json \
    --root-block-votes-dir ./public-root-information/root-block-votes/ \
    --root-commit 0000000000000000000000000000000000000000000000000000000000000000 \
    --genesis-token-supply="1000000000.0" \
    --service-account-public-key-json "{\"PublicKey\":\"R7MTEDdLclRLrj2MI1hcp4ucgRTpR15PCHAWLM5nks6Y3H7+PGkfZTP2di2jbITooWO4DD1yqaBSAVK8iQ6i0A==\",\"SignAlgo\":2,\"HashAlgo\":1,\"SeqNumber\":0,\"Weight\":1000}" \
    -o ./

# allow output to be inspected if necessary
echo "To clean up results, run:"
echo "rm -rf permissionless public-root-information private-root-information execution-state $keygen_dir $partner_dir node-config.json partner-weights.json"
