package crypto

//go:generate rm -rf relic
//go:generate git clone git@github.com:relic-toolkit/relic.git
//go:generate cd relic && git checkout relic-toolkit-0.5.0 && cd ..
//go:generate bash relic_build.sh
