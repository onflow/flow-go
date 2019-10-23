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
  identity=consensus$((i+1))
  identities+=($identity)
  address=localhost:$((i+7297))
  entry=$role'-'$identity'@'$address
  entries+=','$entry
  ((i++))
done

# launch nodes and save process ids
pids=()
entries=${entries:1}
for identity in ${identities[@]}
do
  ./consensus --identity $identity --entries $entries --connections $conns &
  pids+=($!)
done

# trap termination signal to avoid abrupt script exit
trap 'kill -TERM $PID' TERM INT

# wait for nodes to exit; signal is propagated in spite of trapping
for pid in ${pids[@]}
do
  wait $pid
done
