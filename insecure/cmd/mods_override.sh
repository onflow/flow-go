# temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
mv insecure/go.mod insecure/go2.mod

# make a backup of main go.mod, go.sum so it can be reset back to original after corrupt images are built
cp ./go.mod ./go2.mod
cp ./go.sum ./go2.sum

# inject forked libp2p-pubsub into main module to allow building corrupt Docker images
echo "require github.com/yhassanzadeh13/go-libp2p-pubsub v0.6.11-flow-expose-msg.0.20240220190333-03695dea34a3" >> ./go.mod

# update go.sum since added new dependency
go mod tidy
