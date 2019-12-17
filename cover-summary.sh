gocov report cover.json | grep -e '--------' | sed -e "s/^github.com\/dapperlabs\/flow-go\///" -e "s/-//g" > cover-summary
while read line; do
    tcLine="##teamcity[buildStatisticValue"
    arr=($line)
    key=${arr[0]}
    val=${arr[1]%\%}
    tcLine="$tcLine key='coverage-$key' value='$val']"
    echo $tcLine
done <cover-summary
totalArr=($(gocov report cover.json | grep -e "Total Coverage:"))
total=${totalArr[3]}
lines=($(echo $total | tr "/" " "))
echo "##teamcity[buildStatisticValue key='CodeCoverageAbsLCovered' value='${lines[0]#(}']"
echo "##teamcity[buildStatisticValue key='CodeCoverageAbsLTotal' value='${lines[1]%)}']"
