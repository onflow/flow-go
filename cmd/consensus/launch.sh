#!/bin/bash

# number of nodes to launch (default 4)
num=${1:-4}
conns=${2:-3}

# generate the identity table with one identity per node
i=0
entries=""
identities=()
while [ $i -lt $num ]
do
  role=consensus
  nodeid=$(hexdump -n 32 -e '8/4 "%08x"' /dev/urandom)
  nodeids+=($nodeid)
  address=localhost:313$((i+1))
  stake='1000'
  entry=$role'-'$nodeid'@'$address'='$stake
  entries+=','$entry
  ((i++))
done
# launch nodes and save process ids
pids=()
entries=${entries:1}
for nodeid in ${nodeids[@]}
do
  datadir=$(mktemp -d -t flow-data-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX)
  ./consensus --nodeid $nodeid --entries $entries --connections $conns --datadir $datadir &
  pids+=($!)
done

function stop() {
  for pid in ${pids[@]}
  do
    wait $pid
  done
}

# trap termination signal to avoid abrupt script exit
trap stop SIGINT

# wait for nodes to exit; signal is propagated in spite of trapping
for pid in ${pids[@]}
do
  wait $pid
done
