#!/bin/bash

###
#  This file generates a set of STDOUT outputs that teamcity picks up and charts as statistics.
#  It takes the output of `gocov test` or `gocov convert` (e.g. gocov's JSON interchange format) and prints out a summary
###


# Add all the sub coverages to the cover file
tail -n +2 crypto/$COVER_PROFILE >> $COVER_PROFILE
gocov convert $COVER_PROFILE > cover.json

# The summary lines are indicated by a long running set of `-`s, therefore to get the summary lines, we grep the lines with a set of dashes
# To remove noise, we remove the root project path `github.com/onflow/flow-go` and all the dashes using `sed`
# Then we print out the lines in a way that teamcity understands, e.g. `##teamcity[buildStatisticValue key='package' value='100.00]'`
gocov report cover.json | grep -e '--------' | sed -e "s/^github.com\/onflow\/flow-go\///" -e "s/-//g" > cover-summary
while read line; do
    tcLine="##teamcity[buildStatisticValue"
    eval 'arr=($line)'
    key=${arr[0]}
    val=${arr[1]%\%}
    tcLine="$tcLine key='coverage-$key' value='$val']"
    echo $tcLine
done <cover-summary

# Also, we can make use of the total repo coverage as well. Done by `grep`ing  the `Total Coverage` line.
# Team city has specific keys for this coverage, e.g. CodeCoverageAbsLCovered and CodeCoverageAbsLTotal
eval 'totalArr=($(gocov report cover.json | grep -e "Total Coverage:"))'
total=${totalArr[3]}
eval 'lines=($(echo $total | tr "/" " "))'
echo "##teamcity[buildStatisticValue key='CodeCoverageAbsLCovered' value='${lines[0]#(}']"
echo "##teamcity[buildStatisticValue key='CodeCoverageAbsLTotal' value='${lines[1]%)}']"
