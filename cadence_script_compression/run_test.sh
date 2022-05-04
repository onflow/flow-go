#!/bin/bash

for num in 1 2 3 4 5
do
mkdir -p ./test_run_output/out"${num}"
go test ./brotli/main_test.go -v > ./test_run_output/out"${num}"/brotli.out
go test ./lz4/main_test.go -v > ./test_run_output/out"${num}"/lz4.out
go test ./snappy/main_test.go -v > ./test_run_output/out"${num}"/snappy.out
go test ./zstd/datadog_zstd/main_test.go -v > ./test_run_output/out"${num}"/datadog_zstd.out
go test ./zstd/klauspost_compress/main_test.go -v > ./test_run_output/out"${num}"/klauspost_zstd.out
go test ./zstd/valyala_gozstd/main_test.go -v > ./test_run_output/out"${num}"/valyala_gozstd.out
done

tar -cf test_run_outputs.tar ./test_run_output
