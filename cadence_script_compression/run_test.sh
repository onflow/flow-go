#!/bin/bash

mkdir ./out"$1"
go test ./brotli/main_test.go -v > ./out"$1"/brotli.out
go test ./lz4/main_test.go -v > ./out"$1"/lz4.out
go test ./snappy/main_test.go -v > ./out"$1"/snappy.out
go test ./zstd/datadog_zstd/main_test.go -v > ./out"$1"/datadog_zstd.out
go test ./zstd/klauspost_compress/main_test.go -v > ./out"$1"/klauspost_zstd.out
go test ./zstd/valyala_gozstd/main_test.go -v > ./out"$1"/valyala_gozstd.out
