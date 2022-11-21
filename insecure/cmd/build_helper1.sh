# temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
mv ../go.mod ../go2.mod

echo "require github.com/yhassanzadeh13/go-libp2p-pubsub v0.6.12-0.20221110181155-60457b3ef6d5" >> ../../go.mod
