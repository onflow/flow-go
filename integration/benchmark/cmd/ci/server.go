package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/onflow/flow-go/integration/benchmark/proto"
)

type benchmarkServer struct {
	pb.UnimplementedBenchmarkServer
}

func (s *benchmarkServer) StartMacroBenchmark(*pb.StartMacroBenchmarkRequest, pb.Benchmark_StartMacroBenchmarkServer) error {
	return status.Errorf(codes.Unimplemented, "method StartMacroBenchmark not implemented")
}
func (s *benchmarkServer) GetMacroBenchmark(context.Context, *pb.GetMacroBenchmarkRequest) (*pb.GetMacroBenchmarkResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMacroBenchmark not implemented")
}
func (s *benchmarkServer) ListMacroBenchmarks(context.Context, *emptypb.Empty) (*pb.ListMacroBenchmarksResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListMacroBenchmarks not implemented")
}
func (s *benchmarkServer) Status(context.Context, *emptypb.Empty) (*pb.StatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Status not implemented")
}
