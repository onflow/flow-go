# go get github.com/gogo/protobuf/protoc-gen-gofast

.PHONY: generate
generate:
		protoc --gofast_out=. *.proto

.PHONY: generate
clean:
		rm -f *.pb.go
