
#!/bin/bash

rm -rf relic

# clone a specif version of Relic without history if it's tagged.
#git clone --branch <tag> --single-branch --depth 1 git@github.com:relic-toolkit/relic.git

# clone all the history if the verison is only defined by a commit hash.
git clone --branch master --single-branch git@github.com:relic-toolkit/relic.git
cd relic
git checkout 7a9bba7
cd ..

# build relic
bash relic_build.sh