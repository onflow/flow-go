#!/bin/bash

go test ./brotli/main_test.go -v > brotli.out
go test ./lz4/main_test.go -v > lz4.out
go test ./snappy/main_test.go -v > snappy.out
go test ./zstd/datadog_zstd/main_test.go -v > datadog_zstd.out
go test ./zstd/klauspost_compress/main_test.go -v > klauspost_zstd.out
go test ./zstd/valyala_gozstd/main_test.go -v > valyala_gozstd.out
