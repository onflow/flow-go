# reset insecure module to original after building corrupt Docker images
mv insecure/go2.mod insecure/go.mod

# reset main module to original after building corrupt Docker images
mv ./go2.mod ./go.mod
mv ./go2.sum ./go.sum
