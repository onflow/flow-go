#!/bin/bash

mkdir ./out
go test ./brotli/main_test.go -v > ./out/brotli.out
go test ./lz4/main_test.go -v > ./out/lz4.out
go test ./snappy/main_test.go -v > ./out/snappy.out
go test ./zstd/datadog_zstd/main_test.go -v > ./out/datadog_zstd.out
go test ./zstd/klauspost_compress/main_test.go -v > ./out/klauspost_zstd.out
go test ./zstd/valyala_gozstd/main_test.go -v > ./out/valyala_gozstd.out
