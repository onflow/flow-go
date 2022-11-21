# temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
mv insecure/go.mod insecure/go2.mod

# make a backup of main go.mod so it can be reset back to original after corrupt images are built
cp ./go.mod ./go2.mod

# inject forked libp2p-pubsub into main module to allow building corrupt Docker images
echo "require github.com/yhassanzadeh13/go-libp2p-pubsub v0.6.12-0.20221110181155-60457b3ef6d5" >> ./go.mod
