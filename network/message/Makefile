# To re-generate the the protobuf go code, install tools first:
# ```
# cd flow-go
# make install-tools
# ```
# Install protoc:
# https://grpc.io/docs/protoc-installation/
#
# Then run:
# ```
# cd network/message
# make generate
# ```


.PHONY: generate
generate:
		protoc --gofast_out=. *.proto

.PHONY: generate
clean:
		rm -f *.pb.go
