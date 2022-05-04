#!/bin/bash

for num in 1 2 3 4 5
do
mkdir ./out"${num}"
go test ./brotli/main_test.go -v > ./out"${num}"/brotli.out
go test ./lz4/main_test.go -v > ./out"${num}"/lz4.out
go test ./snappy/main_test.go -v > ./out"${num}"/snappy.out
go test ./zstd/datadog_zstd/main_test.go -v > ./out"${num}"/datadog_zstd.out
go test ./zstd/klauspost_compress/main_test.go -v > ./out"${num}"/klauspost_zstd.out
go test ./zstd/valyala_gozstd/main_test.go -v > ./out"${num}"/valyala_gozstd.out
done

