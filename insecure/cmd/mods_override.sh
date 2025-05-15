# temporarily make insecure/ a non-module to allow Docker to use corrupt builders there
mv insecure/go.mod insecure/go2.mod

# make a backup of main go.mod, go.sum so it can be reset back to original after corrupt images are built
cp ./go.mod ./go2.mod
cp ./go.sum ./go2.sum

# update go.sum since added new dependency
go mod tidy
